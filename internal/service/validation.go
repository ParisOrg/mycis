package service

import (
	"fmt"
	"net/mail"
	"net/url"
	"strings"

	"mycis/internal/db"
)

const (
	maxEvidenceURLLength    = 2048
	maxEvidenceLabelBytes   = 256
	maxCommentBytes         = 16 * 1024
	maxNotesBytes           = 64 * 1024
	maxBlockedReasonBytes   = 16 * 1024
	maxAssessmentNameBytes  = 512
	maxAssessmentScopeBytes = 4000
	maxUserNameBytes        = 200
	maxEmailBytes           = 320
	minPasswordChars        = 10
)

func errIfTooLong(s string, maxBytes int, field string) error {
	if len(s) > maxBytes {
		return fmt.Errorf("%w: %s is too long (max %d bytes)", ErrInvalidInput, field, maxBytes)
	}
	return nil
}

// ValidateEvidenceURL returns the trimmed URL if it is an absolute https URL with a host.
func ValidateEvidenceURL(raw string) (string, error) {
	if err := errIfTooLong(raw, maxEvidenceURLLength, "evidence URL"); err != nil {
		return "", err
	}
	u, err := url.Parse(raw)
	if err != nil {
		return "", fmt.Errorf("%w: invalid URL", ErrInvalidInput)
	}
	if !strings.EqualFold(u.Scheme, "https") || u.Host == "" || u.Opaque != "" {
		return "", fmt.Errorf("%w: evidence URL must be an https link with a host", ErrInvalidInput)
	}
	if strings.ContainsAny(raw, "\r\n") {
		return "", fmt.Errorf("%w: invalid URL", ErrInvalidInput)
	}
	return raw, nil
}

// ParseAssessmentItemStatus maps a form value to a DB enum.
func ParseAssessmentItemStatus(s string) (db.AssessmentItemStatus, error) {
	switch s {
	case string(db.AssessmentItemStatusNotStarted):
		return db.AssessmentItemStatusNotStarted, nil
	case string(db.AssessmentItemStatusInProgress):
		return db.AssessmentItemStatusInProgress, nil
	case string(db.AssessmentItemStatusReadyForReview):
		return db.AssessmentItemStatusReadyForReview, nil
	case string(db.AssessmentItemStatusValidated):
		return db.AssessmentItemStatusValidated, nil
	case string(db.AssessmentItemStatusNotApplicable):
		return db.AssessmentItemStatusNotApplicable, nil
	case string(db.AssessmentItemStatusBlocked):
		return db.AssessmentItemStatusBlocked, nil
	default:
		return "", fmt.Errorf("%w: invalid status", ErrInvalidInput)
	}
}

// ParseItemPriority maps a form value to a DB enum.
func ParseItemPriority(s string) (db.ItemPriority, error) {
	switch s {
	case string(db.ItemPriorityLow):
		return db.ItemPriorityLow, nil
	case string(db.ItemPriorityMedium):
		return db.ItemPriorityMedium, nil
	case string(db.ItemPriorityHigh):
		return db.ItemPriorityHigh, nil
	case string(db.ItemPriorityCritical):
		return db.ItemPriorityCritical, nil
	default:
		return "", fmt.Errorf("%w: invalid priority", ErrInvalidInput)
	}
}

// ValidateEmailForStorage parses and normalizes an email for create/update flows.
func ValidateEmailForStorage(raw string) (string, error) {
	trimmed := strings.TrimSpace(raw)
	if trimmed == "" {
		return "", fmt.Errorf("%w: email is required", ErrInvalidInput)
	}
	addr, err := mail.ParseAddress(trimmed)
	if err != nil {
		return "", fmt.Errorf("%w: invalid email address", ErrInvalidInput)
	}
	norm := strings.ToLower(addr.Address)
	if err := errIfTooLong(norm, maxEmailBytes, "email"); err != nil {
		return "", err
	}
	return norm, nil
}

// NormalizeEmailForAuth parses email for login; returns ErrUnauthorized on invalid input.
func NormalizeEmailForAuth(raw string) (string, error) {
	trimmed := strings.TrimSpace(raw)
	if trimmed == "" {
		return "", ErrUnauthorized
	}
	addr, err := mail.ParseAddress(trimmed)
	if err != nil {
		return "", ErrUnauthorized
	}
	norm := strings.ToLower(addr.Address)
	if len(norm) > maxEmailBytes {
		return "", ErrUnauthorized
	}
	return norm, nil
}

func normalizePassword(password string) (string, error) {
	password = strings.TrimSpace(password)
	if len(password) < minPasswordChars {
		return "", fmt.Errorf("%w: password must be at least %d characters", ErrInvalidInput, minPasswordChars)
	}
	return password, nil
}
