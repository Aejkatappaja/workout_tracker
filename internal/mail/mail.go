// Package mail sends transactional email behind a swappable Mailer interface.
// The default LogMailer just logs, so the whole flow works in dev without any
// external service; setting RESEND_API_KEY + MAIL_FROM switches to real delivery.
package mail

import (
	"bytes"
	"context"
	"encoding/json"
	"fmt"
	"io"
	"log"
	"net/http"
	"time"
)

// Mailer sends a single transactional message (HTML + plaintext fallback).
type Mailer interface {
	Send(ctx context.Context, to, subject, html, text string) error
}

// LogMailer writes messages to the logger instead of delivering them. Used when
// no provider is configured; the reset link is visible in the server log.
type LogMailer struct{ Logger *log.Logger }

func (m LogMailer) Send(_ context.Context, to, subject, _, text string) error {
	m.Logger.Printf("MAIL (log-only) to=%s subject=%q\n%s", to, subject, text)
	return nil
}

// ResendMailer delivers via the Resend HTTP API. No SDK: a single JSON POST, so
// the dependency footprint stays at the standard library.
type ResendMailer struct {
	apiKey string
	from   string
	client *http.Client
}

func (m ResendMailer) Send(ctx context.Context, to, subject, html, text string) error {
	payload, err := json.Marshal(map[string]any{
		"from":    m.from,
		"to":      []string{to},
		"subject": subject,
		"html":    html,
		"text":    text,
	})
	if err != nil {
		return err
	}

	req, err := http.NewRequestWithContext(ctx, http.MethodPost, "https://api.resend.com/emails", bytes.NewReader(payload))
	if err != nil {
		return err
	}
	req.Header.Set("Authorization", "Bearer "+m.apiKey)
	req.Header.Set("Content-Type", "application/json")

	resp, err := m.client.Do(req)
	if err != nil {
		return err
	}
	defer func() { _ = resp.Body.Close() }()

	if resp.StatusCode >= 300 {
		body, _ := io.ReadAll(io.LimitReader(resp.Body, 2<<10))
		return fmt.Errorf("resend: %s: %s", resp.Status, body)
	}
	return nil
}

// New returns a ResendMailer when both apiKey and from are set, otherwise a
// LogMailer.
func New(logger *log.Logger, apiKey, from string) Mailer {
	if apiKey != "" && from != "" {
		return ResendMailer{apiKey: apiKey, from: from, client: &http.Client{Timeout: 10 * time.Second}}
	}
	logger.Printf("mail: RESEND_API_KEY/MAIL_FROM not set, using log-only mailer")
	return LogMailer{Logger: logger}
}
