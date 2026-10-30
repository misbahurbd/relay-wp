package storage_test

import (
	"context"
	"errors"
	"io"
	"os"
	"strings"
	"testing"

	"github.com/misbahurbd/relay-wp/apps/api/internal/platform/config"
	"github.com/misbahurbd/relay-wp/apps/api/internal/platform/storage"
)

func TestNewRejectsUnknownDriver(t *testing.T) {
	if _, err := storage.New(context.Background(), config.Storage{Driver: "floppy"}, false); err == nil {
		t.Error("expected error for unknown driver")
	}
	if _, err := storage.New(context.Background(), config.Storage{Driver: "s3"}, false); err == nil {
		t.Error("expected error for s3 without bucket and keys")
	}
}

func TestMemory(t *testing.T) {
	st, err := storage.New(context.Background(), config.Storage{Driver: "memory"}, false)
	if err != nil {
		t.Fatal(err)
	}
	roundTrip(t, st)
}

// TestS3 talks to a real S3-compatible server (RustFS locally). Set S3_TEST_ENDPOINT to run it.
func TestS3(t *testing.T) {
	endpoint := os.Getenv("S3_TEST_ENDPOINT")
	if endpoint == "" {
		t.Skip("S3_TEST_ENDPOINT not set")
	}
	st, err := storage.New(context.Background(), config.Storage{
		Driver: "s3", S3Endpoint: endpoint, S3Region: "us-east-1", S3PathStyle: true,
		S3AccessKey: os.Getenv("S3_ACCESS_KEY"), S3SecretKey: os.Getenv("S3_SECRET_KEY"), S3Bucket: "relay-test",
	}, true)
	if err != nil {
		t.Fatal(err)
	}
	roundTrip(t, st)
}

func roundTrip(t *testing.T, st storage.Storage) {
	t.Helper()
	ctx := context.Background()
	const key, body = "tests/hello.txt", "hello"

	// Wrapped so it can't seek: callers pass readers like a bytes.Buffer.
	plain := struct{ io.Reader }{strings.NewReader(body)}
	if err := st.Put(ctx, key, plain, int64(len(body)), "text/plain"); err != nil {
		t.Fatal(err)
	}
	rc, err := st.Get(ctx, key)
	if err != nil {
		t.Fatal(err)
	}
	got, _ := io.ReadAll(rc)
	_ = rc.Close()
	if string(got) != body {
		t.Errorf("Get = %q", got)
	}
	if err := st.Delete(ctx, key); err != nil {
		t.Fatal(err)
	}
	if _, err := st.Get(ctx, key); !errors.Is(err, storage.ErrNotFound) {
		t.Errorf("Get after Delete = %v, want ErrNotFound", err)
	}
}
