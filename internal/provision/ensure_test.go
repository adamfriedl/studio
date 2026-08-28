package provision

import (
	"bytes"
	"context"
	"fmt"
	"strings"
	"testing"

	gh "github.com/adamfriedl/studio/internal/github"
)

func TestPackFilesGoService(t *testing.T) {
	files, err := PackFiles("go-service")
	if err != nil {
		t.Fatal(err)
	}
	ci, ok := files[".github/workflows/ci.yml"]
	if !ok || !bytes.Contains(ci, []byte("name: ci")) || !bytes.Contains(ci, []byte("go test")) {
		t.Fatalf("bad ci.yml: %s", ci)
	}
	hook, ok := files[".github/workflows/studio-hook.yml"]
	if !ok || !bytes.Contains(hook, []byte("name: studio-hook")) {
		t.Fatalf("bad hook: %s", hook)
	}
	if !bytes.Contains(hook, []byte(`workflows: ["ci"]`)) {
		t.Fatalf("hook missing workflow_run for ci: %s", hook)
	}
	if bytes.Contains(hook, []byte("STUDIO_APP_PRIVATE_KEY")) {
		t.Fatal("pack hook must not reference App private key")
	}
}

func TestPackFilesUnknown(t *testing.T) {
	_, err := PackFiles("python-svc")
	if err == nil || !strings.Contains(err.Error(), "unknown provision pack") {
		t.Fatalf("want unknown pack error, got %v", err)
	}
}

type fakeClient struct {
	files   map[string][]byte
	secrets map[string]string
	puts    []string
}

func (f *fakeClient) GetFile(_ context.Context, _, _, path string) (*gh.FileContent, error) {
	b, ok := f.files[path]
	if !ok {
		return nil, fmt.Errorf("github GET: 404 Not Found")
	}
	return &gh.FileContent{SHA: "sha-" + path, Content: b}, nil
}

func (f *fakeClient) PutFile(_ context.Context, _, _, path, _ string, content []byte, _ string) error {
	if f.files == nil {
		f.files = map[string][]byte{}
	}
	f.files[path] = content
	f.puts = append(f.puts, path)
	return nil
}

func (f *fakeClient) PutActionsSecret(_ context.Context, _, _, name, value string) error {
	if f.secrets == nil {
		f.secrets = map[string]string{}
	}
	f.secrets[name] = value
	return nil
}

func TestEnsureMissingHookToken(t *testing.T) {
	t.Setenv("STUDIO_HOOK_TOKEN", "")
	_, err := Ensure(context.Background(), &fakeClient{}, "adamfriedl", "notes-api", "go-service")
	if err == nil || !strings.Contains(err.Error(), "STUDIO_HOOK_TOKEN") {
		t.Fatalf("want hook token error, got %v", err)
	}
}

func TestEnsureUpserts(t *testing.T) {
	t.Setenv("STUDIO_HOOK_TOKEN", "ghp_test_hook_token")
	fc := &fakeClient{files: map[string][]byte{}}
	res, err := Ensure(context.Background(), fc, "adamfriedl", "notes-api", "go-service")
	if err != nil {
		t.Fatal(err)
	}
	if len(res.Files) != 2 || len(res.Secrets) != 1 {
		t.Fatalf("got %#v", res)
	}
	if fc.secrets["STUDIO_HOOK_TOKEN"] != "ghp_test_hook_token" {
		t.Fatalf("secret: %q", fc.secrets["STUDIO_HOOK_TOKEN"])
	}
	if _, ok := fc.secrets["STUDIO_APP_PRIVATE_KEY"]; ok {
		t.Fatal("must not copy App private key onto target")
	}
	if _, ok := fc.files[".github/workflows/ci.yml"]; !ok {
		t.Fatal("ci.yml not written")
	}
	hook := fc.files[".github/workflows/studio-hook.yml"]
	if hook == nil {
		t.Fatal("hook not written")
	}
	if !bytes.Contains(hook, []byte("STUDIO_HOOK_TOKEN")) {
		t.Fatal("hook should use STUDIO_HOOK_TOKEN")
	}
	if bytes.Contains(hook, []byte("STUDIO_APP_PRIVATE_KEY")) {
		t.Fatal("pack hook must not reference App private key")
	}
}
