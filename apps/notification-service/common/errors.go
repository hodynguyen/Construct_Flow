package common

import "errors"

var (
	ErrNotFound        = errors.New("notification not found")
	ErrForbidden       = errors.New("access denied")
	ErrDuplicateEvent  = errors.New("event already processed")
	ErrInvalidPayload  = errors.New("invalid event payload")
)
