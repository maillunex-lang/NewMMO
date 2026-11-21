package quest

import "errors"

var (
	ErrNotFound            = errors.New("NOT_FOUND")
	ErrStateConflict       = errors.New("STATE_CONFLICT")
	ErrPrerequisiteNotMet  = errors.New("PREREQUISITE_NOT_MET")
	ErrLevelTooLow         = errors.New("LEVEL_TOO_LOW")
	ErrUnsupportedEvent    = errors.New("UNSUPPORTED_EVENT")
	ErrInvalidTarget       = errors.New("INVALID_TARGET")
	ErrContentMismatch     = errors.New("CONTENT_MISMATCH")
	ErrIdempotencyConflict = errors.New("IDEMPOTENCY_CONFLICT")
	ErrRegionClosed        = errors.New("REGION_CLOSED")
	ErrBadRequest          = errors.New("BAD_REQUEST")
	ErrInternal            = errors.New("INTERNAL")
)

func Code(err error) string {
	switch {
	case errors.Is(err, ErrNotFound):
		return "NOT_FOUND"
	case errors.Is(err, ErrStateConflict):
		return "STATE_CONFLICT"
	case errors.Is(err, ErrPrerequisiteNotMet):
		return "PREREQUISITE_NOT_MET"
	case errors.Is(err, ErrLevelTooLow):
		return "LEVEL_TOO_LOW"
	case errors.Is(err, ErrUnsupportedEvent):
		return "UNSUPPORTED_EVENT"
	case errors.Is(err, ErrInvalidTarget):
		return "INVALID_TARGET"
	case errors.Is(err, ErrContentMismatch):
		return "CONTENT_MISMATCH"
	case errors.Is(err, ErrIdempotencyConflict):
		return "IDEMPOTENCY_CONFLICT"
	case errors.Is(err, ErrRegionClosed):
		return "REGION_CLOSED"
	case errors.Is(err, ErrBadRequest):
		return "BAD_REQUEST"
	default:
		return "INTERNAL"
	}
}
