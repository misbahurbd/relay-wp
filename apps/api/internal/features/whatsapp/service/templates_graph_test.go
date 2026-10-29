package service

import (
	"context"
	"encoding/json"
	"io"
	"net/http"
	"net/http/httptest"
	"strings"
	"testing"

	"github.com/google/uuid"

	"github.com/misbahurbd/relay-wp/apps/api/internal/features/whatsapp/model"
)

func TestCreateTemplatePayload(t *testing.T) {
	var got map[string]any
	var path, auth string
	srv := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		path, auth = r.URL.Path, r.Header.Get("Authorization")
		_ = json.NewDecoder(r.Body).Decode(&got)
		_, _ = w.Write([]byte(`{"id":"594425479261596","status":"PENDING","category":"MARKETING"}`))
	}))
	defer srv.Close()

	g := NewGraph(srv.Client(), srv.URL+"/v24.0", "app", "secret")
	id, status, err := g.CreateTemplate(t.Context(), "tok", "1001", model.TemplateSpec{
		Name: "summer_sale", Language: "en_US", Category: "marketing",
		Header: model.TemplateHeader{Format: "image", Handle: "4::aW1hZ2U="},
		Body:   model.TemplateBody{Text: "Hi {{1}}, {{2}} off today.", Examples: []string{"Daniel", "20%"}},
		Footer: "Reply STOP to opt out",
		Buttons: []model.TemplateButton{
			{Type: "quick_reply", Text: "Stop promos"},
			{Type: "url", Text: "Shop", Value: "https://acme.com/sale"},
			{Type: "phone", Text: "Call us", Value: "+14155550142"},
		},
	})
	if err != nil {
		t.Fatal(err)
	}
	if id != "594425479261596" || templateStatus(status) != model.TemplatePending {
		t.Fatalf("id %q status %q", id, status)
	}
	if path != "/v24.0/1001/message_templates" || auth != "Bearer tok" {
		t.Fatalf("path %q auth %q", path, auth)
	}

	want := `{
		"name": "summer_sale", "language": "en_US", "category": "marketing", "parameter_format": "positional",
		"components": [
			{"type": "HEADER", "format": "IMAGE", "example": {"header_handle": ["4::aW1hZ2U="]}},
			{"type": "BODY", "text": "Hi {{1}}, {{2}} off today.", "example": {"body_text": [["Daniel", "20%"]]}},
			{"type": "FOOTER", "text": "Reply STOP to opt out"},
			{"type": "BUTTONS", "buttons": [
				{"type": "QUICK_REPLY", "text": "Stop promos"},
				{"type": "URL", "text": "Shop", "url": "https://acme.com/sale"},
				{"type": "PHONE_NUMBER", "text": "Call us", "phone_number": "+14155550142"}
			]}
		]
	}`
	var wantMap map[string]any
	if err := json.Unmarshal([]byte(want), &wantMap); err != nil {
		t.Fatal(err)
	}
	gotJSON, _ := json.Marshal(got)
	wantJSON, _ := json.Marshal(wantMap)
	if string(gotJSON) != string(wantJSON) {
		t.Fatalf("payload\n got %s\nwant %s", gotJSON, wantJSON)
	}
}

func TestAuthenticationTemplatePayload(t *testing.T) {
	tests := []struct {
		name string
		auth model.TemplateAuth
		want string
	}{
		{"copy code", model.TemplateAuth{SecurityRecommendation: true, CodeExpiryMinutes: 10, OTPType: "copy_code", ButtonText: "Copy code"}, `[
			{"type": "BODY", "add_security_recommendation": true},
			{"type": "FOOTER", "code_expiration_minutes": 10},
			{"type": "BUTTONS", "buttons": [{"type": "OTP", "otp_type": "COPY_CODE", "text": "Copy code"}]}
		]`},
		{"one tap", model.TemplateAuth{OTPType: "one_tap", ButtonText: "Copy code", AutofillText: "Autofill", PackageName: "com.acme.app", SignatureHash: "K8a/AINcGX7"}, `[
			{"type": "BODY"},
			{"type": "BUTTONS", "buttons": [{"type": "OTP", "otp_type": "ONE_TAP", "text": "Copy code", "autofill_text": "Autofill",
				"supported_apps": [{"package_name": "com.acme.app", "signature_hash": "K8a/AINcGX7"}]}]}
		]`},
	}
	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			auth := tt.auth
			raw, _ := json.Marshal(metaComponents(model.TemplateSpec{
				Category: "authentication", Body: model.TemplateBody{Text: "ignored"}, Auth: &auth,
			}))
			var gotAny, want any
			if err := json.Unmarshal(raw, &gotAny); err != nil {
				t.Fatal(err)
			}
			if err := json.Unmarshal([]byte(tt.want), &want); err != nil {
				t.Fatal(err)
			}
			got, _ := json.Marshal(gotAny)
			wantJSON, _ := json.Marshal(want)
			if string(got) != string(wantJSON) {
				t.Fatalf("components\n got %s\nwant %s", got, wantJSON)
			}
		})
	}
}

func TestListedAuthenticationTemplate(t *testing.T) {
	var mt metaTemplate
	_ = json.Unmarshal([]byte(`{"id": "9", "name": "otp", "language": "en_US", "status": "APPROVED", "category": "AUTHENTICATION",
		"components": [
			{"type": "BODY", "text": "*{{1}}* is your verification code. For your security, do not share this code."},
			{"type": "FOOTER", "text": "This code expires in 5 minutes.", "code_expiration_minutes": 5},
			{"type": "BUTTONS", "buttons": [{"type": "OTP", "otp_type": "ONE_TAP", "text": "Copy code", "autofill_text": "Autofill",
				"package_name": "com.acme.app", "signature_hash": "K8a/AINcGX7"}]}
		]}`), &mt)
	got, ok := fromMetaTemplate(mt)
	if !ok || got.Spec.Auth == nil {
		t.Fatalf("not converted: %+v", got)
	}
	want := model.TemplateAuth{
		SecurityRecommendation: true, CodeExpiryMinutes: 5, OTPType: "one_tap", ButtonText: "Copy code",
		AutofillText: "Autofill", PackageName: "com.acme.app", SignatureHash: "K8a/AINcGX7",
	}
	if *got.Spec.Auth != want || len(got.Spec.Buttons) != 0 {
		t.Fatalf("auth %+v buttons %+v", *got.Spec.Auth, got.Spec.Buttons)
	}
}

func TestTextHeaderAndNoVariables(t *testing.T) {
	c := metaComponents(model.TemplateSpec{
		Header: model.TemplateHeader{Format: "text", Text: "Order {{1}}", Example: "#4821"},
		Body:   model.TemplateBody{Text: "Your order shipped."},
	})
	if len(c) != 2 || c[0].Format != "TEXT" || c[0].Example.HeaderText[0] != "#4821" || c[1].Example != nil {
		t.Fatalf("components = %+v", c)
	}
}

func TestListTemplatesFollowsPagingAndConverts(t *testing.T) {
	pages := map[string]string{
		"": `{"data":[
			{"id":"1","name":"hello_world","language":"en_US","status":"APPROVED","category":"UTILITY",
			 "components":[{"type":"HEADER","format":"TEXT","text":"Hello World"},{"type":"BODY","text":"Welcome!"},
			   {"type":"FOOTER","text":"Bye"}]},
			{"id":"2","name":"gone","language":"en_US","status":"PENDING_DELETION","category":"UTILITY","components":[]}
		 ],"paging":{"cursors":{"after":"c1"},"next":"https://graph/next"}}`,
		"c1": `{"data":[
			{"id":"3","name":"promo","language":"es","status":"REJECTED","category":"MARKETING","rejected_reason":"INVALID_FORMAT",
			 "components":[{"type":"BODY","text":"Hola {{1}}!","example":{"body_text":[["Ana"]]}},
			   {"type":"BUTTONS","buttons":[{"type":"URL","text":"Ver","url":"https://acme.com"},{"type":"OTP","text":"Copy"}]}]}
		 ],"paging":{"cursors":{"after":"c2"}}}`,
	}
	srv := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		_, _ = io.WriteString(w, pages[r.URL.Query().Get("after")])
	}))
	defer srv.Close()

	list, err := NewGraph(srv.Client(), srv.URL, "app", "secret").ListTemplates(t.Context(), "tok", "1001")
	if err != nil {
		t.Fatal(err)
	}
	if len(list) != 2 {
		t.Fatalf("got %d templates: %+v", len(list), list)
	}
	hw := list[0]
	if hw.Status != model.TemplateApproved || hw.Spec.Category != "utility" || hw.Spec.Header.Text != "Hello World" || hw.Spec.Footer != "Bye" {
		t.Fatalf("hello_world = %+v", hw)
	}
	p := list[1]
	if p.Status != model.TemplateRejected || p.RejectedReason != "INVALID_FORMAT" || p.Spec.Header.Format != "none" ||
		p.Spec.Body.Examples[0] != "Ana" || len(p.Spec.Buttons) != 1 || p.Spec.Buttons[0].Value != "https://acme.com" {
		t.Fatalf("promo = %+v", p)
	}
}

func TestUploadSampleUsesResumableUpload(t *testing.T) {
	var uploadAuth, offset, body string
	srv := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		switch r.URL.Path {
		case "/app/uploads":
			q := r.URL.Query()
			if q.Get("file_length") != "5" || q.Get("file_type") != "image/png" {
				t.Errorf("session query = %v", q)
			}
			_, _ = io.WriteString(w, `{"id":"upload:MTphdHRh?sig=ARZ"}`)
		case "/upload:MTphdHRh":
			uploadAuth, offset = r.Header.Get("Authorization"), r.Header.Get("file_offset")
			b, _ := io.ReadAll(r.Body)
			body = string(b)
			_, _ = io.WriteString(w, `{"h":"4::handle"}`)
		default:
			http.NotFound(w, r)
		}
	}))
	defer srv.Close()

	h, err := NewGraph(srv.Client(), srv.URL, "app", "secret").UploadSample(t.Context(), "tok", "image/png", strings.NewReader("hello"), 5)
	if err != nil {
		t.Fatal(err)
	}
	if h != "4::handle" || uploadAuth != "OAuth tok" || offset != "0" || body != "hello" {
		t.Fatalf("handle %q auth %q offset %q body %q", h, uploadAuth, offset, body)
	}
}

type sinkCall struct {
	ws                 uuid.UUID
	id, status, reason string
}

type recordingSink struct{ calls []sinkCall }

func (r *recordingSink) TemplateStatusChanged(_ context.Context, ws uuid.UUID, id, status, reason string) error {
	r.calls = append(r.calls, sinkCall{ws, id, status, reason})
	return nil
}

func TestTemplateStatusWebhook(t *testing.T) {
	ws := uuid.Must(uuid.NewV7())
	tests := []struct {
		value              string
		wantStatus, reason string // wantStatus "" means ignored
	}{
		{`{"event":"APPROVED","message_template_id":594425479261596,"reason":"NONE"}`, "approved", ""},
		{`{"event":"REJECTED","message_template_id":594425479261596,"reason":"INVALID_FORMAT"}`, "rejected", "INVALID_FORMAT"},
		{`{"event":"REJECTED","message_template_id":"594425479261596","reason":"NONE"}`, "rejected", ""},
		{`{"event":"PAUSED","message_template_id":594425479261596,"reason":"x"}`, "paused", ""},
		{`{"event":"DISABLED","message_template_id":594425479261596}`, "disabled", ""},
		{`{"event":"PENDING","message_template_id":594425479261596}`, "pending", ""},
		{`{"event":"PENDING_DELETION","message_template_id":594425479261596}`, "", ""},
		{`{"event":"APPROVED"}`, "", ""},
	}
	for _, tt := range tests {
		sink := &recordingSink{}
		s := &Service{}
		s.SetTemplateStatusSink(sink)
		if err := s.onTemplateStatus(t.Context(), account{WorkspaceID: ws}, json.RawMessage(tt.value)); err != nil {
			t.Fatal(err)
		}
		if tt.wantStatus == "" {
			if len(sink.calls) != 0 {
				t.Errorf("%s: unexpected call %+v", tt.value, sink.calls)
			}
			continue
		}
		want := sinkCall{ws, "594425479261596", tt.wantStatus, tt.reason}
		if len(sink.calls) != 1 || sink.calls[0] != want {
			t.Errorf("%s: calls %+v, want %+v", tt.value, sink.calls, want)
		}
	}
}
