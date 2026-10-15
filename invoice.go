package main

import (
	"bytes"
	"context"
	"encoding/json"
	"errors"
	"fmt"
	"html"
	"io"
	"net/http"
	"strconv"
	"strings"
	"time"
)

type MatterOrder struct {
	MatterID                string `json:"matter_id"`
	ClientEmail             string `json:"client_email"`
	Description             string `json:"description"`
	AmountCents             int64  `json:"amount_cents"`
	SignedDocumentDelivered bool   `json:"signed_document_delivered"`
	Deadline                string `json:"deadline"`
}

type Decision struct {
	InvoiceHTML string
	FollowUp    bool
}

func decide(o MatterOrder, now time.Time) (Decision, error) {
	if strings.TrimSpace(o.MatterID) == "" || strings.TrimSpace(o.ClientEmail) == "" || o.AmountCents <= 0 {
		return Decision{}, errors.New("matter_id, client_email and positive amount_cents are required")
	}
	deadline, err := time.Parse("2006-01-02", o.Deadline)
	if err != nil {
		return Decision{}, errors.New("deadline must be YYYY-MM-DD")
	}
	if !o.SignedDocumentDelivered {
		return Decision{}, errors.New("signed document must be delivered before invoicing")
	}
	markup := func(s string) string { return html.EscapeString(s) }
	page := fmt.Sprintf("<!doctype html><html><body><h1>Invoice: %s</h1><p>Matter: %s</p><p>%s</p><p>Amount: %d.%02d USD</p><p>Signed document delivered. Deadline: %s</p></body></html>", markup(o.MatterID), markup(o.MatterID), markup(o.Description), o.AmountCents/100, o.AmountCents%100, markup(o.Deadline))
	return Decision{InvoiceHTML: page, FollowUp: !deadline.Before(now.UTC().Truncate(24*time.Hour)) && !deadline.After(now.UTC().Truncate(24*time.Hour).Add(7*24*time.Hour))}, nil
}

type apiError struct {
	Code    string `json:"code"`
	Message string `json:"message"`
}
type envelope struct {
	OK       bool            `json:"ok"`
	Data     json.RawMessage `json:"data"`
	Error    *apiError       `json:"error"`
	Metadata json.RawMessage `json:"metadata"`
}
type remoteError struct {
	Status        int
	Code, Message string
}

func (e remoteError) Error() string { return fmt.Sprintf("Infrai %s: %s", e.Code, e.Message) }

type client struct {
	key, base string
	http      *http.Client
}

func (c client) post(ctx context.Context, path string, payload any, token string) (json.RawMessage, error) {
	body, err := json.Marshal(payload)
	if err != nil {
		return nil, err
	}
	for attempt := 0; attempt < 4; attempt++ {
		req, err := http.NewRequestWithContext(ctx, http.MethodPost, c.base+path, bytes.NewReader(body))
		if err != nil {
			return nil, err
		}
		req.Header.Set("Authorization", "Bearer "+c.key)
		req.Header.Set("Content-Type", "application/json")
		req.Header.Set("Idempotency-Key", token)
		resp, err := c.http.Do(req)
		if err != nil {
			return nil, err
		}
		raw, err := io.ReadAll(io.LimitReader(resp.Body, 2<<20))
		resp.Body.Close()
		if err != nil {
			return nil, err
		}
		var env envelope
		if err := json.Unmarshal(raw, &env); err != nil {
			return nil, fmt.Errorf("decode Infrai response: %w", err)
		}
		if resp.StatusCode == http.StatusTooManyRequests && attempt < 3 {
			pause := time.Duration(1<<attempt) * time.Second
			if seconds, err := strconv.Atoi(resp.Header.Get("Retry-After")); err == nil && seconds >= 0 {
				pause = time.Duration(seconds) * time.Second
			}
			select {
			case <-time.After(pause):
			case <-ctx.Done():
				return nil, ctx.Err()
			}
			continue
		}
		if !env.OK {
			if env.Error != nil {
				return nil, remoteError{resp.StatusCode, env.Error.Code, env.Error.Message}
			}
			return nil, fmt.Errorf("Infrai rejected request (HTTP %d)", resp.StatusCode)
		}
		if resp.StatusCode >= 500 {
			return nil, fmt.Errorf("Infrai HTTP %d", resp.StatusCode)
		}
		return env.Data, nil
	}
	return nil, errors.New("retry budget exhausted")
}

func (c client) process(ctx context.Context, o MatterOrder, now time.Time) (map[string]any, error) {
	d, err := decide(o, now)
	if err != nil {
		return nil, err
	}
	// One key and base URL serve PDF rendering and email delivery.
	pdf, err := c.post(ctx, "/v1/pdf/generate", map[string]any{"html": d.InvoiceHTML, "page_size": "A4", "store": false}, "invoice-pdf:"+o.MatterID)
	if err != nil {
		return nil, err
	}
	mail, err := c.post(ctx, "/v1/email/send", map[string]any{"to": o.ClientEmail, "subject": "Invoice for matter " + o.MatterID, "html": d.InvoiceHTML}, "invoice-email:"+o.MatterID)
	if err != nil {
		return nil, err
	}
	var sent struct {
		MessageID string `json:"message_id"`
	}
	if err := json.Unmarshal(mail, &sent); err != nil {
		return nil, err
	}
	result := map[string]any{"matter_id": o.MatterID, "pdf": json.RawMessage(pdf), "message_id": sent.MessageID, "deadline_follow_up_due": d.FollowUp}
	return result, nil
}
