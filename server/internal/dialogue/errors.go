package dialogue

import "errors"

var (
	ErrNotFound           = errors.New("NOT_FOUND")
	ErrPrerequisiteNotMet = errors.New("PREREQUISITE_NOT_MET")
	ErrStateConflict      = errors.New("STATE_CONFLICT")
	ErrBadRequest         = errors.New("BAD_REQUEST")
	ErrIdempotencyConflict = errors.New("IDEMPOTENCY_CONFLICT")
	ErrInternal           = errors.New("INTERNAL")
)

func Code(err error) string {
	switch {
	case errors.Is(err, ErrNotFound):
		return "NOT_FOUND"
	case errors.Is(err, ErrPrerequisiteNotMet):
		return "PREREQUISITE_NOT_MET"
	case errors.Is(err, ErrStateConflict):
		return "STATE_CONFLICT"
	case errors.Is(err, ErrBadRequest):
		return "BAD_REQUEST"
	case errors.Is(err, ErrIdempotencyConflict):
		return "IDEMPOTENCY_CONFLICT"
	default:
		return "INTERNAL"
	}
}
