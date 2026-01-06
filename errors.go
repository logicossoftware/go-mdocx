package mdocx

import "errors"

// Sentinel errors returned by Encode and Decode functions.
// These errors can be checked using errors.Is for programmatic error handling.
var (
	// ErrInvalidMagic indicates the file does not have valid MDOCX magic bytes.
	// This typically means the file is not an MDOCX file or is corrupted.
	ErrInvalidMagic = errors.New("mdocx: invalid magic")

	// ErrUnsupportedVersion indicates the file uses an unsupported format version.
	// This package only supports VersionV1.
	ErrUnsupportedVersion = errors.New("mdocx: unsupported version")

	// ErrInvalidHeader indicates the fixed header is malformed or contains invalid values.
	// This includes incorrect header size, non-zero reserved fields, or missing flags.
	ErrInvalidHeader = errors.New("mdocx: invalid fixed header")

	// ErrInvalidSection indicates a section header is malformed or unexpected.
	// This includes wrong section type, unknown compression, or non-zero reserved fields.
	ErrInvalidSection = errors.New("mdocx: invalid section header")

	// ErrInvalidPayload indicates section payload data is malformed.
	// This includes decompression errors, wrong entry names in ZIP, or gob decode failures.
	ErrInvalidPayload = errors.New("mdocx: invalid payload")

	// ErrLimitExceeded indicates a configured size limit was exceeded.
	// Use WithReadLimits or WithWriteLimits to adjust limits.
	ErrLimitExceeded = errors.New("mdocx: limit exceeded")

	// ErrValidation indicates document validation failed.
	// This includes missing required fields, duplicate paths/IDs, invalid paths, or SHA256 mismatches.
	ErrValidation = errors.New("mdocx: validation failed")
)
