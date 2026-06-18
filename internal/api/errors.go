package api

import (
	"encoding/json"
	"errors"
	"fmt"
	"net/http"
)

// Error is a structured API error carrying the HTTP status, the upstream
// message, and supporting metadata. The command layer maps StatusCode to a
// process exit code.
type Error struct {
	StatusCode int
	Message    string
	Code       string // OpusClip error code, e.g. API_MONTHLY_CAP_REACHED
	RequestID  string
	RetryAfter string // Retry-After header value (429)
	CapReason  string // X-Cap-Reason header, e.g. "concurrent"
	ResetAt    string // monthly-cap reset time (from body)
	UpgradeURL string // monthly-cap upgrade link (from body)
	Body       string
}

func (e *Error) Error() string {
	msg := e.Message
	if msg == "" {
		msg = http.StatusText(e.StatusCode)
	}
	if e.Code != "" {
		return fmt.Sprintf("API error %d (%s): %s", e.StatusCode, e.Code, msg)
	}
	return fmt.Sprintf("API error %d: %s", e.StatusCode, msg)
}

// newAPIError parses a non-2xx response into an Error, enriching it with the
// OpusClip-specific signals (rate-limit, concurrency cap, monthly cap) so the
// command layer and user get an actionable message.
func newAPIError(status int, headers http.Header, body []byte) *Error {
	e := &Error{StatusCode: status, Body: string(body)}
	if headers != nil {
		e.RequestID = headers.Get("X-Request-Id")
		e.RetryAfter = headers.Get("Retry-After")
		e.CapReason = headers.Get("X-Cap-Reason")
	}

	var parsed struct {
		Message    string `json:"message"`
		Error      string `json:"error"`
		Detail     string `json:"detail"`
		Code       string `json:"code"`
		ResetAt    string `json:"reset_at"`
		UpgradeURL string `json:"upgrade_url"`
	}
	if json.Unmarshal(body, &parsed) == nil {
		e.Code = parsed.Code
		e.ResetAt = parsed.ResetAt
		e.UpgradeURL = parsed.UpgradeURL
		switch {
		case parsed.Message != "":
			e.Message = parsed.Message
		case parsed.Error != "":
			e.Message = parsed.Error
		case parsed.Detail != "":
			e.Message = parsed.Detail
		}
	}

	// Craft actionable messages for the gating responses.
	switch {
	case status == http.StatusForbidden && e.Code == "API_MONTHLY_CAP_REACHED":
		e.Message = "monthly clip cap reached"
		if e.ResetAt != "" {
			e.Message += " — resets at " + e.ResetAt
		}
		if e.UpgradeURL != "" {
			e.Message += " (upgrade: " + e.UpgradeURL + ")"
		}
	case status == http.StatusTooManyRequests && e.CapReason == "concurrent":
		e.Message = "concurrency cap reached — too many projects rendering; retry shortly"
	case status == http.StatusTooManyRequests && e.Message == "":
		e.Message = "rate limit (30/min) exceeded"
		if e.RetryAfter != "" {
			e.Message += " — retry after " + e.RetryAfter + "s"
		}
	}
	return e
}

// asError is a thin wrapper around errors.As used by tests.
func asError(err error, target **Error) bool {
	return errors.As(err, target)
}
