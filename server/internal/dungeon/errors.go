package dungeon

import "errors"

var (
	ErrBadRequest          = errors.New("BAD_REQUEST")
	ErrNotFound            = errors.New("NOT_FOUND")
	ErrStateConflict       = errors.New("STATE_CONFLICT")
	ErrInvalidTarget       = errors.New("INVALID_TARGET")
	ErrLevelTooLow         = errors.New("LEVEL_TOO_LOW")
	ErrRegionClosed        = errors.New("REGION_CLOSED")
	ErrIdempotencyConflict = errors.New("IDEMPOTENCY_CONFLICT")
	ErrInternal            = errors.New("INTERNAL")
)
