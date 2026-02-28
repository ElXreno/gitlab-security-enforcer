package handler

import (
	"context"
	"errors"
	"io"
	"log/slog"
	"net/http"
	"strings"
	"testing"

	"github.com/gofiber/fiber/v2"
)

type mockEnforcer struct {
	calls []int
	err   error
}

func (m *mockEnforcer) SecureProject(_ context.Context, projectID int) error {
	m.calls = append(m.calls, projectID)
	return m.err
}

func newTestHandler(enforcer ProjectSecurityEnforcer) *fiber.App {
	logger := slog.New(slog.NewJSONHandler(io.Discard, nil))
	return NewApp("test-secret", enforcer, logger)
}

func TestHealthz(t *testing.T) {
	handler := newTestHandler(&mockEnforcer{})
	req, err := http.NewRequest(http.MethodGet, "/healthz", nil)
	if err != nil {
		t.Fatalf("failed to build request: %v", err)
	}

	rec, err := handler.Test(req)
	if err != nil {
		t.Fatalf("request failed: %v", err)
	}
	defer rec.Body.Close()
	body, _ := io.ReadAll(rec.Body)

	if rec.StatusCode != http.StatusOK {
		t.Fatalf("expected status 200, got %d", rec.StatusCode)
	}
	if strings.TrimSpace(string(body)) != "ok" {
		t.Fatalf("expected body ok, got %q", string(body))
	}
}

func TestSystemHookMissingTokenReturnsForbidden(t *testing.T) {
	handler := newTestHandler(&mockEnforcer{})
	req, err := http.NewRequest(http.MethodPost, "/", strings.NewReader(`{"event_name":"project_create","project_id":1}`))
	if err != nil {
		t.Fatalf("failed to build request: %v", err)
	}

	rec, err := handler.Test(req)
	if err != nil {
		t.Fatalf("request failed: %v", err)
	}

	if rec.StatusCode != http.StatusForbidden {
		t.Fatalf("expected status 403, got %d", rec.StatusCode)
	}
}

func TestSystemHookWrongTokenReturnsForbidden(t *testing.T) {
	handler := newTestHandler(&mockEnforcer{})
	req, err := http.NewRequest(http.MethodPost, "/", strings.NewReader(`{"event_name":"project_create","project_id":1}`))
	if err != nil {
		t.Fatalf("failed to build request: %v", err)
	}
	req.Header.Set("X-Gitlab-Token", "wrong")

	rec, err := handler.Test(req)
	if err != nil {
		t.Fatalf("request failed: %v", err)
	}

	if rec.StatusCode != http.StatusForbidden {
		t.Fatalf("expected status 403, got %d", rec.StatusCode)
	}
}

func TestSystemHookNonProjectEventReturnsOK(t *testing.T) {
	enforcer := &mockEnforcer{}
	handler := newTestHandler(enforcer)
	req, err := http.NewRequest(http.MethodPost, "/", strings.NewReader(`{"event_name":"user_create","user_id":1}`))
	if err != nil {
		t.Fatalf("failed to build request: %v", err)
	}
	req.Header.Set("X-Gitlab-Token", "test-secret")
	req.Header.Set("Content-Type", "application/json")

	rec, err := handler.Test(req)
	if err != nil {
		t.Fatalf("request failed: %v", err)
	}

	if rec.StatusCode != http.StatusOK {
		t.Fatalf("expected status 200, got %d", rec.StatusCode)
	}
	if len(enforcer.calls) != 0 {
		t.Fatalf("expected secureProject not to be called, got %d calls", len(enforcer.calls))
	}
}

func TestSystemHookProjectCreateStillReturnsOKWhenEnforcerFails(t *testing.T) {
	enforcer := &mockEnforcer{err: errors.New("boom")}
	handler := newTestHandler(enforcer)
	req, err := http.NewRequest(http.MethodPost, "/", strings.NewReader(`{"event_name":"project_create","project_id":42}`))
	if err != nil {
		t.Fatalf("failed to build request: %v", err)
	}
	req.Header.Set("X-Gitlab-Token", "test-secret")
	req.Header.Set("Content-Type", "application/json")

	rec, err := handler.Test(req)
	if err != nil {
		t.Fatalf("request failed: %v", err)
	}

	if rec.StatusCode != http.StatusOK {
		t.Fatalf("expected status 200, got %d", rec.StatusCode)
	}
	if len(enforcer.calls) != 1 || enforcer.calls[0] != 42 {
		t.Fatalf("expected secureProject to be called once with project_id 42, got %+v", enforcer.calls)
	}
}
