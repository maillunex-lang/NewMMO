package inventory

import "errors"

var (
	ErrNotFound            = errors.New("NOT_FOUND")
	ErrStateConflict       = errors.New("STATE_CONFLICT")
	ErrBadRequest          = errors.New("BAD_REQUEST")
	ErrIdempotencyConflict = errors.New("IDEMPOTENCY_CONFLICT")
	ErrInternal            = errors.New("INTERNAL")
	ErrLevelTooLow         = errors.New("LEVEL_TOO_LOW")
	ErrWrongClass          = errors.New("WRONG_CLASS")
)

func Code(err error) string {
	switch {
	case errors.Is(err, ErrNotFound):
		return "NOT_FOUND"
	case errors.Is(err, ErrStateConflict):
		return "STATE_CONFLICT"
	case errors.Is(err, ErrBadRequest):
		return "BAD_REQUEST"
	case errors.Is(err, ErrLevelTooLow):
		return "LEVEL_TOO_LOW"
	case errors.Is(err, ErrWrongClass):
		return "WRONG_CLASS"
	case errors.Is(err, ErrIdempotencyConflict):
		return "IDEMPOTENCY_CONFLICT"
	default:
		return "INTERNAL"
	}
}
