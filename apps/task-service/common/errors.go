package common

import "errors"

var (
	ErrNotFound                = errors.New("resource not found")
	ErrTaskLocked              = errors.New("task is being modified by another request, please try again")
	ErrInvalidStatusTransition = errors.New("invalid task status transition")
	ErrForbidden               = errors.New("insufficient permissions for this operation")
	ErrUserNotFound            = errors.New("user not found in this company")
	ErrInvalidInput            = errors.New("invalid input")
	ErrAlreadyExists           = errors.New("resource already exists")
)
