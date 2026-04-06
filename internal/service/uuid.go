package service

import (
	"fmt"

	"github.com/google/uuid"
)

func uuidFromString(value string) (uuid.UUID, error) {
	id, err := uuid.Parse(value)
	if err != nil {
		return uuid.UUID{}, fmt.Errorf("%w: invalid uuid", ErrInvalidInput)
	}
	return id, nil
}

func uuidString(value *uuid.UUID) *string {
	if value == nil {
		return nil
	}
	text := value.String()
	return &text
}
