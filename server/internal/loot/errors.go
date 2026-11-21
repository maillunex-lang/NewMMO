package loot

import "errors"

var (
	ErrNotFound            = errors.New("NOT_FOUND")
	ErrBadRequest          = errors.New("BAD_REQUEST")
	ErrRegionClosed        = errors.New("REGION_CLOSED")
	ErrRateLimited         = errors.New("RATE_LIMITED")
	ErrUnsupported         = errors.New("UNSUPPORTED_EVENT")
	ErrIdempotencyConflict = errors.New("IDEMPOTENCY_CONFLICT")
	ErrInternal            = errors.New("INTERNAL")
)
