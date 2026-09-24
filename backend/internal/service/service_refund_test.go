package service

import (
	"strings"
	"testing"
	"time"
)

func TestRefundApplyReasonRequired(t *testing.T) {
	svc := &RefundService{}
	tests := []struct {
		name   string
		reason string
	}{
		{name: "empty reason", reason: ""},
		{name: "blank reason", reason: "   "},
	}
	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			_, _, err := svc.Apply(1, 1, ApplyInput{Reason: tt.reason})
			if err == nil || !strings.Contains(err.Error(), "reason") {
				t.Fatalf("expected reason required error, got %v", err)
			}
		})
	}
}

func TestRefundWindowExpired(t *testing.T) {
	now := time.Now()
	tests := []struct {
		name      string
		donatedAt time.Time
		want      bool
	}{
		{name: "just donated", donatedAt: now, want: false},
		{name: "within two days", donatedAt: now.Add(-47 * time.Hour), want: false},
		{name: "exactly two days", donatedAt: now.Add(-48 * time.Hour), want: false},
		{name: "beyond two days", donatedAt: now.Add(-48*time.Hour - time.Minute), want: true},
		{name: "long past", donatedAt: now.Add(-7 * 24 * time.Hour), want: true},
	}
	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			if got := refundWindowExpired(tt.donatedAt, now); got != tt.want {
				t.Fatalf("refundWindowExpired(%v, %v) = %v, want %v", tt.donatedAt, now, got, tt.want)
			}
		})
	}
}

func TestRefundReviewStatusValidation(t *testing.T) {
	svc := &RefundService{}
	tests := []struct {
		name   string
		status string
	}{
		{name: "pending not allowed", status: "pending"},
		{name: "unknown status", status: "unknown"},
		{name: "empty status", status: ""},
	}
	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			_, err := svc.Review(1, 1, ReviewInput{Status: tt.status})
			if err == nil || !strings.Contains(err.Error(), "invalid review status") {
				t.Fatalf("expected invalid review status error, got %v", err)
			}
		})
	}
}
