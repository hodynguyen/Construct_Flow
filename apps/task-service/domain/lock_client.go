package domain

import (
	"context"
	"time"
)

// LockClient abstracts distributed locking operations.
// Keeping Redis out of the domain layer lets us mock it in unit tests.
type LockClient interface {
	SetNX(ctx context.Context, key string, value interface{}, expiration time.Duration) (bool, error)
	Del(ctx context.Context, keys ...string) error
}
