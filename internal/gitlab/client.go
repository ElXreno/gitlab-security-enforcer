package gitlab

import (
	"bytes"
	"context"
	"encoding/json"
	"fmt"
	"io"
	"log/slog"
	"net/http"
	"net/url"
	"strconv"
	"strings"
	"time"
)

const requestTimeout = 10 * time.Second

var securitySettingsRetryBackoffs = []time.Duration{
	250 * time.Millisecond,
	500 * time.Millisecond,
	1 * time.Second,
	2 * time.Second,
}

type Client struct {
	baseURL    string
	token      string
	httpClient *http.Client
	logger     *slog.Logger
}

func New(baseURL, token string, logger *slog.Logger) (*Client, error) {
	parsed, err := url.Parse(strings.TrimRight(baseURL, "/"))
	if err != nil || parsed.Scheme == "" || parsed.Host == "" {
		return nil, fmt.Errorf("invalid GITLAB_URL: %q", baseURL)
	}

	return &Client{
		baseURL:    strings.TrimRight(baseURL, "/"),
		token:      token,
		httpClient: &http.Client{},
		logger:     logger,
	}, nil
}

func (c *Client) SecureProject(ctx context.Context, projectID int) error {
	if err := c.enableSecretPushProtection(ctx, projectID); err != nil {
		return err
	}

	if err := c.enablePreventSecretsPushRule(ctx, projectID); err != nil {
		return err
	}

	c.logger.Info("successfully enforced project security settings", "project_id", projectID)
	return nil
}

func (c *Client) enableSecretPushProtection(ctx context.Context, projectID int) error {
	path := "/api/v4/projects/" + strconv.Itoa(projectID) + "/security_settings"
	payload := map[string]bool{
		"secret_push_protection_enabled": true,
	}

	_, err := c.doJSONWithRetry(ctx, http.MethodPut, path, payload, securitySettingsRetryBackoffs)
	if err != nil {
		c.logger.Error("failed to enable secret push protection", "project_id", projectID, "error", err)
		return err
	}

	return nil
}

func (c *Client) enablePreventSecretsPushRule(ctx context.Context, projectID int) error {
	path := "/api/v4/projects/" + strconv.Itoa(projectID) + "/push_rule"
	payload := map[string]bool{
		"prevent_secrets": true,
	}

	statusCode, err := c.doJSON(ctx, http.MethodPut, path, payload)
	if err == nil {
		return nil
	}

	if statusCode != http.StatusNotFound {
		c.logger.Error("failed to update push rule", "project_id", projectID, "error", err)
		return err
	}

	c.logger.Info("push rule not found on PUT, retrying with POST", "project_id", projectID)
	_, err = c.doJSON(ctx, http.MethodPost, path, payload)
	if err != nil {
		c.logger.Error("failed to create push rule with POST", "project_id", projectID, "error", err)
		return err
	}
	c.logger.Info("successfully created push rule with POST fallback", "project_id", projectID)

	return nil
}

func (c *Client) doJSON(ctx context.Context, method, path string, payload any) (int, error) {
	requestBody, err := json.Marshal(payload)
	if err != nil {
		return 0, fmt.Errorf("marshal payload: %w", err)
	}

	timeoutCtx, cancel := context.WithTimeout(ctx, requestTimeout)
	defer cancel()

	req, err := http.NewRequestWithContext(timeoutCtx, method, c.baseURL+path, bytes.NewReader(requestBody))
	if err != nil {
		return 0, fmt.Errorf("create request: %w", err)
	}
	req.Header.Set("Private-Token", c.token)
	req.Header.Set("Content-Type", "application/json")

	resp, err := c.httpClient.Do(req)
	if err != nil {
		return 0, fmt.Errorf("perform request: %w", err)
	}
	defer resp.Body.Close()

	if resp.StatusCode >= http.StatusOK && resp.StatusCode < http.StatusMultipleChoices {
		return resp.StatusCode, nil
	}

	body, _ := io.ReadAll(io.LimitReader(resp.Body, 2048))
	return resp.StatusCode, fmt.Errorf("gitlab api %s %s returned %d: %s", method, path, resp.StatusCode, strings.TrimSpace(string(body)))
}

func (c *Client) doJSONWithRetry(ctx context.Context, method, path string, payload any, backoffs []time.Duration) (int, error) {
	lastStatus := 0
	lastErr := error(nil)

	for attempt := 0; attempt <= len(backoffs); attempt++ {
		statusCode, err := c.doJSON(ctx, method, path, payload)
		if err == nil {
			return statusCode, nil
		}

		lastStatus = statusCode
		lastErr = err

		if !isRetryableStatus(statusCode) || attempt == len(backoffs) {
			break
		}

		delay := backoffs[attempt]
		c.logger.Warn("gitlab call failed, retrying", "method", method, "path", path, "status_code", statusCode, "attempt", attempt+1, "next_retry_in", delay.String())

		timer := time.NewTimer(delay)
		select {
		case <-ctx.Done():
			timer.Stop()
			return lastStatus, fmt.Errorf("context canceled while waiting to retry %s %s: %w", method, path, ctx.Err())
		case <-timer.C:
		}
	}

	return lastStatus, lastErr
}

func isRetryableStatus(statusCode int) bool {
	return statusCode == http.StatusNotFound ||
		statusCode == http.StatusConflict ||
		statusCode == http.StatusTooManyRequests ||
		statusCode >= http.StatusInternalServerError
}
