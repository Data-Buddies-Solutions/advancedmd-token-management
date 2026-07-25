package scheduling_test

import (
	"testing"
	"time"

	"advancedmd-token-management/internal/domain"
	"advancedmd-token-management/internal/scheduling"
)

func TestSlotTokenRejectsTamperingWrongSecretAndUnsafeTimes(t *testing.T) {
	now := time.Date(2026, 6, 1, 12, 0, 0, 0, time.UTC)
	policy := scheduling.SlotPolicy{
		OfficeID:      "spring_hill",
		Routing:       string(domain.RoutingAll),
		ColumnID:      1513,
		ProfileID:     620,
		StartDatetime: "2026-06-02T09:00",
		Duration:      15,
		IssuedAt:      now.Unix(),
		ExpiresAt:     now.Add(15 * time.Minute).Unix(),
	}
	token, err := scheduling.SignSlotToken("test-booking-secret", policy)
	if err != nil {
		t.Fatalf("SignSlotToken error = %v", err)
	}

	tests := []struct {
		name   string
		secret string
		token  string
		now    time.Time
	}{
		{name: "wrong secret", secret: "wrong-secret", token: token, now: now},
		{name: "tampered", secret: "test-booking-secret", token: token + "x", now: now},
		{name: "expired", secret: "test-booking-secret", token: token, now: now.Add(15 * time.Minute)},
	}
	for _, test := range tests {
		t.Run(test.name, func(t *testing.T) {
			if _, err := scheduling.VerifySlotToken(test.secret, test.token, test.now); err == nil {
				t.Fatal("VerifySlotToken error = nil")
			}
		})
	}

	oversizedTTL := policy
	oversizedTTL.ExpiresAt = now.Add(16 * time.Minute).Unix()
	oversizedToken, err := scheduling.SignSlotToken("test-booking-secret", oversizedTTL)
	if err != nil {
		t.Fatalf("SignSlotToken oversized TTL error = %v", err)
	}
	if _, err := scheduling.VerifySlotToken("test-booking-secret", oversizedToken, now); err == nil {
		t.Fatal("VerifySlotToken accepted oversized TTL")
	}

	futureIssued := policy
	futureIssued.IssuedAt = now.Add(3 * time.Minute).Unix()
	futureIssued.ExpiresAt = now.Add(10 * time.Minute).Unix()
	futureToken, err := scheduling.SignSlotToken("test-booking-secret", futureIssued)
	if err != nil {
		t.Fatalf("SignSlotToken future issue error = %v", err)
	}
	if _, err := scheduling.VerifySlotToken("test-booking-secret", futureToken, now); err == nil {
		t.Fatal("VerifySlotToken accepted future issuance")
	}
}
