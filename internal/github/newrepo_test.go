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

func TestCreateFromTemplate(t *testing.T) {
	var generated bool
	srv := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		switch {
		case r.Method == http.MethodGet && r.URL.Path == "/repos/adamfriedl/notes-api":
			w.WriteHeader(http.StatusNotFound)
		case r.Method == http.MethodPost && r.URL.Path == "/repos/adamfriedl/template-go-svc/generate":
			generated = true
			var body map[string]any
			_ = json.NewDecoder(r.Body).Decode(&body)
			if body["name"] != "notes-api" || body["owner"] != "adamfriedl" {
				t.Errorf("bad generate body: %#v", body)
			}
			_ = json.NewEncoder(w).Encode(Repository{
				Name:     "notes-api",
				FullName: "adamfriedl/notes-api",
				HTMLURL:  "https://github.com/adamfriedl/notes-api",
				Private:  true,
			})
		default:
			t.Errorf("unexpected %s %s", r.Method, r.URL.Path)
			w.WriteHeader(http.StatusNotFound)
		}
	}))
	defer srv.Close()

	c := &Client{Token: "t", BaseURL: srv.URL}
	repo, err := c.CreateFromTemplate(context.Background(), "adamfriedl/template-go-svc", "adamfriedl", "notes-api", true)
	if err != nil {
		t.Fatal(err)
	}
	if !generated || repo.FullName != "adamfriedl/notes-api" {
		t.Fatalf("got %#v generated=%v", repo, generated)
	}
}

func TestCreateFromTemplateExists(t *testing.T) {
	srv := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		if r.Method == http.MethodGet && r.URL.Path == "/repos/adamfriedl/notes-api" {
			w.WriteHeader(http.StatusOK)
			_, _ = w.Write([]byte(`{}`))
			return
		}
		t.Errorf("unexpected %s %s", r.Method, r.URL.Path)
		w.WriteHeader(http.StatusInternalServerError)
	}))
	defer srv.Close()

	c := &Client{Token: "t", BaseURL: srv.URL}
	repo, err := c.CreateFromTemplate(context.Background(), "adamfriedl/template-go-svc", "adamfriedl", "notes-api", true)
	if err != ErrRepoExists {
		t.Fatalf("want ErrRepoExists, got %v", err)
	}
	if repo.FullName != "adamfriedl/notes-api" {
		t.Fatalf("got %#v", repo)
	}
}

func TestEnsureLabel(t *testing.T) {
	created := false
	srv := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		switch {
		case r.Method == http.MethodGet && strings.HasSuffix(r.URL.Path, "/labels/repo:notes-api"):
			w.WriteHeader(http.StatusNotFound)
		case r.Method == http.MethodPost && strings.HasSuffix(r.URL.Path, "/labels"):
			created = true
			w.WriteHeader(http.StatusCreated)
			_, _ = w.Write([]byte(`{}`))
		default:
			t.Errorf("unexpected %s %s", r.Method, r.URL.Path)
			w.WriteHeader(http.StatusNotFound)
		}
	}))
	defer srv.Close()

	c := &Client{Token: "t", BaseURL: srv.URL}
	if err := c.EnsureLabel(context.Background(), "adamfriedl", "studio", "repo:notes-api", "0E8A16", "Target: notes-api"); err != nil {
		t.Fatal(err)
	}
	if !created {
		t.Fatal("expected create")
	}
}

func TestGetPutFile(t *testing.T) {
	var putBody map[string]any
	srv := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		switch {
		case r.Method == http.MethodGet && strings.HasSuffix(r.URL.Path, "/contents/repos.yaml"):
			_ = json.NewEncoder(w).Encode(map[string]string{
				"sha":      "abc",
				"encoding": "base64",
				"content":  base64.StdEncoding.EncodeToString([]byte("org: x\nrepos:\n")),
			})
		case r.Method == http.MethodPut && strings.HasSuffix(r.URL.Path, "/contents/repos.yaml"):
			_ = json.NewDecoder(r.Body).Decode(&putBody)
			w.WriteHeader(http.StatusOK)
			_, _ = w.Write([]byte(`{}`))
		default:
			t.Errorf("unexpected %s %s", r.Method, r.URL.Path)
			w.WriteHeader(http.StatusNotFound)
		}
	}))
	defer srv.Close()

	c := &Client{Token: "t", BaseURL: srv.URL}
	f, err := c.GetFile(context.Background(), "adamfriedl", "studio", "repos.yaml")
	if err != nil {
		t.Fatal(err)
	}
	if f.SHA != "abc" || string(f.Content) != "org: x\nrepos:\n" {
		t.Fatalf("got %#v", f)
	}
	if err := c.PutFile(context.Background(), "adamfriedl", "studio", "repos.yaml", "chore: allowlist notes-api", []byte("org: x\nrepos:\n  - name: notes-api\n"), f.SHA); err != nil {
		t.Fatal(err)
	}
	if putBody["sha"] != "abc" {
		t.Fatalf("put body: %#v", putBody)
	}
}
