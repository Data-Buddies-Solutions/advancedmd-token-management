package scheduling

import (
	"crypto/hmac"
	"crypto/sha256"
	"encoding/base64"
	"encoding/json"
	"errors"
	"strconv"
	"strings"
	"time"

	"advancedmd-token-management/internal/domain"
)

const (
	cancellationTokenVersion = 1
	cancellationTokenPurpose = "appointment_cancellation"
	cancellationTokenTTL     = 15 * time.Minute
	cancellationTokenDomain  = "scheduling-token/appointment-cancellation/v1\x00"
)

var (
	ErrCancellationTokenSecretMissing = errors.New("cancellation token secret is not configured")
	ErrCancellationTokenInvalid       = errors.New("invalid cancellation token")
	ErrCancellationTokenExpired       = errors.New("cancellation token expired")
)

type cancellationPolicy struct {
	Version       int    `json:"v"`
	Purpose       string `json:"purpose"`
	PatientID     string `json:"patientId"`
	AppointmentID int    `json:"appointmentId"`
	OfficeID      string `json:"officeId"`
	Start         string `json:"start"`
	IssuedAt      int64  `json:"iat"`
	ExpiresAt     int64  `json:"exp"`

	start time.Time
}

// CancellationTokens signs the trusted context returned with resolved
// appointments. The same scheduling secret is cryptographically separated
// from booking tokens by a cancellation-only HMAC domain.
type CancellationTokens struct {
	secret string
	now    func() time.Time
}

func NewCancellationTokens(secret string, now func() time.Time) *CancellationTokens {
	if now == nil {
		now = time.Now
	}
	return &CancellationTokens{secret: secret, now: now}
}

func (t *CancellationTokens) IssueCancellationToken(
	patientID string,
	appointment domain.PatientAppointment,
) (string, error) {
	if t == nil || t.secret == "" {
		return "", ErrCancellationTokenSecretMissing
	}
	patientID = domain.StripPatientPrefix(strings.TrimSpace(patientID))
	if _, err := strconv.Atoi(patientID); err != nil ||
		appointment.ID <= 0 ||
		appointment.OfficeID == "" ||
		appointment.Start.IsZero() {
		return "", ErrCancellationTokenInvalid
	}
	if _, ok := domain.LookupOfficeByID(appointment.OfficeID); !ok {
		return "", ErrCancellationTokenInvalid
	}

	now := t.now().UTC()
	policy := cancellationPolicy{
		Version:       cancellationTokenVersion,
		Purpose:       cancellationTokenPurpose,
		PatientID:     patientID,
		AppointmentID: appointment.ID,
		OfficeID:      appointment.OfficeID,
		Start:         appointment.Start.UTC().Format(time.RFC3339),
		IssuedAt:      now.Unix(),
		ExpiresAt:     now.Add(cancellationTokenTTL).Unix(),
	}
	body, err := json.Marshal(policy)
	if err != nil {
		return "", ErrCancellationTokenInvalid
	}
	encodedBody := base64.RawURLEncoding.EncodeToString(body)
	mac := hmac.New(sha256.New, []byte(t.secret))
	mac.Write([]byte(cancellationTokenDomain))
	mac.Write([]byte(encodedBody))
	signature := base64.RawURLEncoding.EncodeToString(mac.Sum(nil))
	return encodedBody + "." + signature, nil
}

func (t *CancellationTokens) verify(token string, now time.Time) (cancellationPolicy, error) {
	if t == nil || t.secret == "" {
		return cancellationPolicy{}, ErrCancellationTokenSecretMissing
	}
	parts := strings.Split(token, ".")
	if len(parts) != 2 || parts[0] == "" || parts[1] == "" {
		return cancellationPolicy{}, ErrCancellationTokenInvalid
	}

	mac := hmac.New(sha256.New, []byte(t.secret))
	mac.Write([]byte(cancellationTokenDomain))
	mac.Write([]byte(parts[0]))
	signature, err := base64.RawURLEncoding.DecodeString(parts[1])
	if err != nil || !hmac.Equal(signature, mac.Sum(nil)) {
		return cancellationPolicy{}, ErrCancellationTokenInvalid
	}
	body, err := base64.RawURLEncoding.DecodeString(parts[0])
	if err != nil {
		return cancellationPolicy{}, ErrCancellationTokenInvalid
	}
	var policy cancellationPolicy
	if err := json.Unmarshal(body, &policy); err != nil {
		return cancellationPolicy{}, ErrCancellationTokenInvalid
	}
	start, err := time.Parse(time.RFC3339, policy.Start)
	if err != nil ||
		policy.Version != cancellationTokenVersion ||
		policy.Purpose != cancellationTokenPurpose ||
		policy.PatientID == "" ||
		policy.AppointmentID <= 0 ||
		policy.OfficeID == "" ||
		policy.IssuedAt <= 0 ||
		policy.ExpiresAt <= 0 {
		return cancellationPolicy{}, ErrCancellationTokenInvalid
	}
	if _, err := strconv.Atoi(policy.PatientID); err != nil {
		return cancellationPolicy{}, ErrCancellationTokenInvalid
	}
	if _, ok := domain.LookupOfficeByID(policy.OfficeID); !ok || start.IsZero() {
		return cancellationPolicy{}, ErrCancellationTokenInvalid
	}

	issuedAt := time.Unix(policy.IssuedAt, 0)
	expiresAt := time.Unix(policy.ExpiresAt, 0)
	if !expiresAt.After(issuedAt) ||
		expiresAt.Sub(issuedAt) > cancellationTokenTTL ||
		issuedAt.After(now.Add(slotTokenClockSkew)) {
		return cancellationPolicy{}, ErrCancellationTokenInvalid
	}
	if !now.Before(expiresAt) {
		return cancellationPolicy{}, ErrCancellationTokenExpired
	}
	policy.start = start
	return policy, nil
}
