package github

import (
	"context"
	"encoding/base64"
	"encoding/json"
	"net/http"
	"net/http/httptest"
	"strings"
	"testing"
)

func TestSealActionsSecret(t *testing.T) {
	// Fixed 32-byte key (not a real GitHub key — only checks length + encoding).
	pk := base64.StdEncoding.EncodeToString(make([]byte, 32))
	enc, err := SealActionsSecret(pk, "super-secret")
	if err != nil {
		t.Fatal(err)
	}
	raw, err := base64.StdEncoding.DecodeString(enc)
	if err != nil {
		t.Fatal(err)
	}
	// ephemeral pub (32) + ciphertext (msg + overhead)
	if len(raw) < 32+len("super-secret") {
		t.Fatalf("ciphertext too short: %d", len(raw))
	}
}

func TestSealActionsSecretBadKey(t *testing.T) {
	_, err := SealActionsSecret(base64.StdEncoding.EncodeToString([]byte("short")), "x")
	if err == nil {
		t.Fatal("expected error")
	}
}

func TestPutActionsSecret(t *testing.T) {
	var putBody map[string]string
	srv := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		switch {
		case r.Method == http.MethodGet && strings.HasSuffix(r.URL.Path, "/actions/secrets/public-key"):
			_ = json.NewEncoder(w).Encode(ActionsPublicKey{
				KeyID: "key-1",
				Key:   base64.StdEncoding.EncodeToString(make([]byte, 32)),
			})
		case r.Method == http.MethodPut && strings.HasSuffix(r.URL.Path, "/actions/secrets/STUDIO_APP_ID"):
			_ = json.NewDecoder(r.Body).Decode(&putBody)
			w.WriteHeader(http.StatusCreated)
		default:
			t.Errorf("unexpected %s %s", r.Method, r.URL.Path)
			w.WriteHeader(http.StatusNotFound)
		}
	}))
	defer srv.Close()

	c := &Client{Token: "t", BaseURL: srv.URL}
	if err := c.PutActionsSecret(context.Background(), "adamfriedl", "notes-api", "STUDIO_APP_ID", "42"); err != nil {
		t.Fatal(err)
	}
	if putBody["key_id"] != "key-1" || putBody["encrypted_value"] == "" {
		t.Fatalf("bad put body: %#v", putBody)
	}
	if _, err := base64.StdEncoding.DecodeString(putBody["encrypted_value"]); err != nil {
		t.Fatal(err)
	}
}
