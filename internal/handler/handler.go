package handler

import (
	"context"
	"log/slog"
	"strings"

	"github.com/gofiber/fiber/v2"
)

type ProjectSecurityEnforcer interface {
	SecureProject(ctx context.Context, projectID int) error
}

type systemHookHandler struct {
	hookSecret string
	enforcer   ProjectSecurityEnforcer
	logger     *slog.Logger
}

type systemHookEvent struct {
	EventName string `json:"event_name"`
	ProjectID int    `json:"project_id"`
}

func newSystemHookHandler(hookSecret string, enforcer ProjectSecurityEnforcer, logger *slog.Logger) *systemHookHandler {
	return &systemHookHandler{
		hookSecret: hookSecret,
		enforcer:   enforcer,
		logger:     logger,
	}
}

func NewApp(hookSecret string, enforcer ProjectSecurityEnforcer, logger *slog.Logger) *fiber.App {
	h := newSystemHookHandler(hookSecret, enforcer, logger)
	app := fiber.New()
	app.Get("/healthz", h.handleHealthz)
	app.Post("/", h.handleSystemHook)
	return app
}

func (h *systemHookHandler) handleHealthz(c *fiber.Ctx) error {
	return c.Status(fiber.StatusOK).SendString("ok")
}

func (h *systemHookHandler) handleSystemHook(c *fiber.Ctx) error {
	if strings.TrimSpace(c.Get("X-Gitlab-Token")) != strings.TrimSpace(h.hookSecret) {
		return c.Status(fiber.StatusForbidden).SendString("forbidden")
	}

	var event systemHookEvent
	if err := c.BodyParser(&event); err != nil {
		return c.Status(fiber.StatusBadRequest).SendString("invalid json body")
	}

	if event.EventName == "project_create" && event.ProjectID > 0 {
		if err := h.enforcer.SecureProject(c.UserContext(), event.ProjectID); err != nil {
			h.logger.Error("failed to enforce security settings", "project_id", event.ProjectID, "error", err)
		}
	}

	return c.SendStatus(fiber.StatusOK)
}
