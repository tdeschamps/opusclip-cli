package api

import (
	"net/http"
	"strings"
	"testing"
)

func TestNewAPIErrorMonthlyCap(t *testing.T) {
	body := []byte(`{"code":"API_MONTHLY_CAP_REACHED","reset_at":"2026-07-01","upgrade_url":"https://opus.pro/upgrade"}`)
	e := newAPIError(http.StatusForbidden, http.Header{"X-Request-Id": {"req-1"}}, body)
	if e.Code != "API_MONTHLY_CAP_REACHED" {
		t.Errorf("code = %q", e.Code)
	}
	if e.ResetAt != "2026-07-01" || e.UpgradeURL == "" {
		t.Errorf("cap fields = %+v", e)
	}
	if e.RequestID != "req-1" {
		t.Errorf("requestID = %q", e.RequestID)
	}
	if msg := e.Error(); msg == "" || !strings.Contains(msg, "monthly clip cap") {
		t.Errorf("message = %q", msg)
	}
}

func TestNewAPIErrorConcurrency(t *testing.T) {
	h := http.Header{"X-Cap-Reason": {"concurrent"}}
	e := newAPIError(http.StatusTooManyRequests, h, nil)
	if e.CapReason != "concurrent" {
		t.Errorf("capReason = %q", e.CapReason)
	}
	if !strings.Contains(e.Error(), "concurrency cap") {
		t.Errorf("message = %q", e.Error())
	}
}

func TestNewAPIErrorRateLimit(t *testing.T) {
	h := http.Header{"Retry-After": {"30"}}
	e := newAPIError(http.StatusTooManyRequests, h, nil)
	if e.RetryAfter != "30" {
		t.Errorf("retryAfter = %q", e.RetryAfter)
	}
	if !strings.Contains(e.Error(), "rate limit") {
		t.Errorf("message = %q", e.Error())
	}
}

func TestAsErrorUnwraps(t *testing.T) {
	e := newAPIError(http.StatusNotFound, nil, []byte(`{"message":"nope"}`))
	var target *Error
	if !asError(error(e), &target) || target.StatusCode != http.StatusNotFound {
		t.Errorf("asError failed: %+v", target)
	}
}
