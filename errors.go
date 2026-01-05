package mdocx

import "errors"

var (
	ErrInvalidMagic       = errors.New("mdocx: invalid magic")
	ErrUnsupportedVersion = errors.New("mdocx: unsupported version")
	ErrInvalidHeader      = errors.New("mdocx: invalid fixed header")
	ErrInvalidSection     = errors.New("mdocx: invalid section header")
	ErrInvalidPayload     = errors.New("mdocx: invalid payload")
	ErrLimitExceeded      = errors.New("mdocx: limit exceeded")
	ErrValidation         = errors.New("mdocx: validation failed")
)
