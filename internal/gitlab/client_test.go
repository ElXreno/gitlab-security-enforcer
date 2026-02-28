package gitlab

import (
	"context"
	"encoding/json"
	"io"
	"log/slog"
	"net/http"
	"net/http/httptest"
	"slices"
	"sync/atomic"
	"testing"
	"time"
)

func newTestGitLabClient(t *testing.T, serverURL string) *Client {
	t.Helper()

	logger := slog.New(slog.NewJSONHandler(io.Discard, nil))
	client, err := New(serverURL, "test-token", logger)
	if err != nil {
		t.Fatalf("failed to create client: %v", err)
	}
	return client
}

func TestSecureProjectSuccess(t *testing.T) {
	var called []string

	ts := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		called = append(called, r.Method+" "+r.URL.Path)

		if got := r.Header.Get("Private-Token"); got != "test-token" {
			t.Fatalf("expected Private-Token header to be set, got %q", got)
		}

		var payload map[string]bool
		if err := json.NewDecoder(r.Body).Decode(&payload); err != nil {
			t.Fatalf("failed to decode payload: %v", err)
		}

		switch r.URL.Path {
		case "/api/v4/projects/42/security_settings":
			if !payload["secret_push_protection_enabled"] {
				t.Fatalf("expected secret_push_protection_enabled=true, got %v", payload)
			}
			w.WriteHeader(http.StatusOK)
		case "/api/v4/projects/42/push_rule":
			if !payload["prevent_secrets"] {
				t.Fatalf("expected prevent_secrets=true, got %v", payload)
			}
			w.WriteHeader(http.StatusOK)
		default:
			t.Fatalf("unexpected path: %s", r.URL.Path)
		}
	}))
	defer ts.Close()

	client := newTestGitLabClient(t, ts.URL)
	if err := client.SecureProject(context.Background(), 42); err != nil {
		t.Fatalf("secureProject failed: %v", err)
	}

	expected := []string{
		"PUT /api/v4/projects/42/security_settings",
		"PUT /api/v4/projects/42/push_rule",
	}
	if !slices.Equal(called, expected) {
		t.Fatalf("expected calls %v, got %v", expected, called)
	}
}

func TestSecureProjectPushRulePut404FallsBackToPost(t *testing.T) {
	var called []string

	ts := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		called = append(called, r.Method+" "+r.URL.Path)

		switch r.Method + " " + r.URL.Path {
		case "PUT /api/v4/projects/7/security_settings":
			w.WriteHeader(http.StatusOK)
		case "PUT /api/v4/projects/7/push_rule":
			w.WriteHeader(http.StatusNotFound)
			_, _ = w.Write([]byte("not found"))
		case "POST /api/v4/projects/7/push_rule":
			var payload map[string]bool
			if err := json.NewDecoder(r.Body).Decode(&payload); err != nil {
				t.Fatalf("failed to decode payload: %v", err)
			}
			if !payload["prevent_secrets"] {
				t.Fatalf("expected prevent_secrets=true, got %v", payload)
			}
			w.WriteHeader(http.StatusCreated)
		default:
			t.Fatalf("unexpected call: %s %s", r.Method, r.URL.Path)
		}
	}))
	defer ts.Close()

	client := newTestGitLabClient(t, ts.URL)
	if err := client.SecureProject(context.Background(), 7); err != nil {
		t.Fatalf("secureProject failed: %v", err)
	}

	expected := []string{
		"PUT /api/v4/projects/7/security_settings",
		"PUT /api/v4/projects/7/push_rule",
		"POST /api/v4/projects/7/push_rule",
	}
	if !slices.Equal(called, expected) {
		t.Fatalf("expected calls %v, got %v", expected, called)
	}
}

func TestSecureProjectRetriesTransient404OnSecuritySettings(t *testing.T) {
	originalBackoffs := securitySettingsRetryBackoffs
	securitySettingsRetryBackoffs = []time.Duration{time.Millisecond, time.Millisecond}
	t.Cleanup(func() {
		securitySettingsRetryBackoffs = originalBackoffs
	})

	var securityAttempts int32
	var called []string

	ts := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		called = append(called, r.Method+" "+r.URL.Path)

		switch r.Method + " " + r.URL.Path {
		case "PUT /api/v4/projects/11/security_settings":
			attempt := atomic.AddInt32(&securityAttempts, 1)
			if attempt == 1 {
				w.WriteHeader(http.StatusNotFound)
				_, _ = w.Write([]byte(`{"message":"404 Project Not Found"}`))
				return
			}
			w.WriteHeader(http.StatusOK)
		case "PUT /api/v4/projects/11/push_rule":
			w.WriteHeader(http.StatusOK)
		default:
			t.Fatalf("unexpected call: %s %s", r.Method, r.URL.Path)
		}
	}))
	defer ts.Close()

	client := newTestGitLabClient(t, ts.URL)
	if err := client.SecureProject(context.Background(), 11); err != nil {
		t.Fatalf("secureProject failed: %v", err)
	}

	expected := []string{
		"PUT /api/v4/projects/11/security_settings",
		"PUT /api/v4/projects/11/security_settings",
		"PUT /api/v4/projects/11/push_rule",
	}
	if !slices.Equal(called, expected) {
		t.Fatalf("expected calls %v, got %v", expected, called)
	}
}
