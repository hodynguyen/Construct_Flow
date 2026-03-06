package common

import "errors"

var (
	ErrNotFound        = errors.New("not found")
	ErrForbidden       = errors.New("forbidden")
	ErrInvalidInput    = errors.New("invalid input")
	ErrGlacierRestore  = errors.New("file is in Glacier — restore required before download")
)
