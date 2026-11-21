package content

import "errors"

type Code string

const (
	BundleInvalid     Code = "BUNDLE_INVALID"
	BundleTooLarge    Code = "BUNDLE_TOO_LARGE"
	HashMismatch      Code = "HASH_MISMATCH"
	SchemaUnsupported Code = "SCHEMA_UNSUPPORTED"
	CountMismatch     Code = "COUNT_MISMATCH"
	Conflict          Code = "CONFLICT"
	NotFound          Code = "NOT_FOUND"
	InvalidState      Code = "INVALID_STATE"
	DBError           Code = "DB_ERROR"
)

type RegistryError struct {
	Code    Code
	Message string
}

func (e *RegistryError) Error() string { return string(e.Code) + ": " + e.Message }

func failure(code Code, message string) error {
	return &RegistryError{Code: code, Message: message}
}

func ErrorCode(err error) Code {
	var e *RegistryError
	if errors.As(err, &e) {
		return e.Code
	}
	return DBError
}
