package orca

import (
	"bytes"
	"context"
	"encoding/json"
	"errors"
	"fmt"
	"html"
	"io"
	"net/http"
	"strings"
	"time"
)

var ErrProviderUnavailable = errors.New("provider is not configured")

func (a *API) sendResendEmail(ctx context.Context, to, subject, htmlBody, idempotencyKey string) error {
	if strings.TrimSpace(a.Config.ResendAPIKey) == "" {
		return ErrProviderUnavailable
	}
	from := strings.TrimSpace(a.Config.EmailFrom)
	if from == "" {
		return errors.New("email sender is not configured")
	}
	body, err := json.Marshal(map[string]any{"from": from, "to": []string{to}, "subject": subject, "html": htmlBody})
	if err != nil {
		return err
	}
	req, err := http.NewRequestWithContext(ctx, http.MethodPost, "https://api.resend.com/emails", bytes.NewReader(body))
	if err != nil {
		return err
	}
	req.Header.Set("Authorization", "Bearer "+a.Config.ResendAPIKey)
	req.Header.Set("Content-Type", "application/json")
	if idempotencyKey != "" {
		req.Header.Set("Idempotency-Key", idempotencyKey)
	}
	client := a.resendClient
	if client == nil {
		client = &http.Client{Timeout: 12 * time.Second}
	}
	resp, err := client.Do(req)
	if err != nil {
		return err
	}
	defer resp.Body.Close()
	if resp.StatusCode < 200 || resp.StatusCode >= 300 {
		_, _ = io.Copy(io.Discard, io.LimitReader(resp.Body, 1024))
		return fmt.Errorf("email provider returned status %d", resp.StatusCode)
	}
	return nil
}

func safeEmailText(value string) string { return html.EscapeString(value) }
