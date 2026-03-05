package common

import "errors"

var (
	ErrNotFound       = errors.New("resource not found")
	ErrEmailExists    = errors.New("email already registered in this company")
	ErrInvalidCredentials = errors.New("invalid email or password")
	ErrInvalidInput   = errors.New("invalid input")
	ErrCompanyNotFound = errors.New("company not found")
	ErrForbidden      = errors.New("insufficient permissions")
)
