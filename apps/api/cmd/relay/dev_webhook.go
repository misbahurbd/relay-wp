package main

import (
	"bytes"
	"context"
	"crypto/hmac"
	"crypto/sha256"
	"encoding/hex"
	"encoding/json"
	"errors"
	"flag"
	"fmt"
	"io"
	"net"
	"net/http"
	"os"
	"strconv"
	"strings"
	"time"

	"github.com/google/uuid"

	"github.com/misbahurbd/relay-wp/apps/api/internal/platform/config"
)

// devWebhook signs a Meta-shaped webhook with META_APP_SECRET and posts it to
// the local API, so the real webhook path runs without a tunnel. By default it
// targets the WABA and number `relay dev seed` gives each sample workspace.
func devWebhook(ctx context.Context, cfg config.Config, args []string, out io.Writer) error {
	fs := flag.NewFlagSet("dev webhook", flag.ContinueOnError)
	fs.SetOutput(out)
	from := fs.String("from", "+14155550101", "the customer's phone, E.164")
	name := fs.String("name", "Daniel Brooks", "the customer's WhatsApp profile name")
	waba := fs.String("waba", "", "WABA ID (default: the seeded one, seed-<workspace-id>)")
	phoneNumberID := fs.String("phone-number-id", "", "Meta phone_number_id (default: the seeded one, seed-<workspace-id>)")
	file := fs.String("file", "", "send this saved webhook body as is instead of building a text message")
	target := fs.String("url", "", "webhook URL (default: http://localhost<HTTP_ADDR port>/v1/webhooks/whatsapp)")
	if err := fs.Parse(args); err != nil {
		return fmt.Errorf("parse flags: %w", err)
	}
	if cfg.WhatsApp.AppSecret == "" {
		return errors.New("set META_APP_SECRET (any value works locally) and WHATSAPP_ENABLED=true")
	}

	var body []byte
	if *file != "" {
		b, err := os.ReadFile(*file)
		if err != nil {
			return fmt.Errorf("read payload: %w", err)
		}
		body = b
	} else {
		workspaceID, text, err := workspaceAndText(fs.Args(), true)
		if err != nil {
			return err
		}
		seeded := "seed-" + workspaceID.String()
		if *waba == "" {
			*waba = seeded
		}
		if *phoneNumberID == "" {
			*phoneNumberID = seeded
		}
		if body, err = textWebhook(*waba, *phoneNumberID, *from, *name, text); err != nil {
			return err
		}
	}

	if *target == "" {
		_, port, err := net.SplitHostPort(cfg.HTTPAddr)
		if err != nil {
			return fmt.Errorf("HTTP_ADDR: %w", err)
		}
		*target = "http://localhost:" + port + "/v1/webhooks/whatsapp"
	}
	mac := hmac.New(sha256.New, []byte(cfg.WhatsApp.AppSecret))
	mac.Write(body)
	req, err := http.NewRequestWithContext(ctx, http.MethodPost, *target, bytes.NewReader(body))
	if err != nil {
		return fmt.Errorf("build request: %w", err)
	}
	req.Header.Set("Content-Type", "application/json")
	req.Header.Set("X-Hub-Signature-256", "sha256="+hex.EncodeToString(mac.Sum(nil)))
	res, err := http.DefaultClient.Do(req)
	if err != nil {
		return fmt.Errorf("post webhook (is the API running?): %w", err)
	}
	defer func() { _ = res.Body.Close() }()
	reply, err := io.ReadAll(io.LimitReader(res.Body, 4096))
	if err != nil {
		return fmt.Errorf("read response: %w", err)
	}
	if res.StatusCode != http.StatusOK {
		return fmt.Errorf("webhook answered %s: %s", res.Status, strings.TrimSpace(string(reply)))
	}
	return say(out, "Webhook accepted (%s). A worker processes it in a moment.\n", res.Status)
}

// textWebhook is a Cloud API "messages" webhook carrying one text message.
func textWebhook(waba, phoneNumberID, from, name, text string) ([]byte, error) {
	digits := strings.TrimPrefix(from, "+")
	body, err := json.Marshal(map[string]any{
		"object": "whatsapp_business_account",
		"entry": []any{map[string]any{
			"id": waba,
			"changes": []any{map[string]any{
				"field": "messages",
				"value": map[string]any{
					"messaging_product": "whatsapp",
					"metadata":          map[string]string{"phone_number_id": phoneNumberID},
					"contacts":          []any{map[string]any{"wa_id": digits, "profile": map[string]string{"name": name}}},
					"messages": []any{map[string]any{
						"from": digits, "id": "wamid.dev." + uuid.Must(uuid.NewV7()).String(),
						"timestamp": strconv.FormatInt(time.Now().Unix(), 10),
						"type":      "text", "text": map[string]string{"body": text},
					}},
				},
			}},
		}},
	})
	if err != nil {
		return nil, fmt.Errorf("encode webhook: %w", err)
	}
	return body, nil
}
