package mdocx // import "github.com/logicossoftware/go-mdocx"

# Package mdocx implements the MDOCX (MarkDown Open Container eXchange) container format.

MDOCX is a single-file container format for bundling one or more Markdown
documents together with referenced binary media (images, audio, video, etc.).
It is designed for exchange, archival, and transport of rich Markdown content.

# File Format Overview

An MDOCX file consists of:

- A 32-byte fixed header with magic bytes, version, and flags
- An optional UTF-8 JSON metadata block
- A Markdown bundle section containing one or more Markdown files
- A Media bundle section containing zero or more media items

Payloads are serialized using Go's encoding/gob and optionally compressed using
ZIP, Zstandard, LZ4, or Brotli compression.

# Basic Usage

To create and write an MDOCX file:

```go
doc := &mdocx.Document{
	Metadata: map[string]any{"title": "My Document"},
	Markdown: mdocx.MarkdownBundle{
		BundleVersion: mdocx.VersionV1,
		Files: []mdocx.MarkdownFile{
			{Path: "readme.md", Content: []byte("# Hello World")},
		},
	},
	Media: mdocx.MediaBundle{BundleVersion: mdocx.VersionV1},
}
f, _ := os.Create("output.mdocx")
defer f.Close()
err := mdocx.Encode(f, doc)
```

To read an MDOCX file:

```go
f, _ := os.Open("input.mdocx")
defer f.Close()
doc, err := mdocx.Decode(f)
```

# Security Considerations

The package includes built-in protection against oversized allocations and
decompression bombs via configurable Limits. All size limits are enforced during
decoding to prevent resource exhaustion attacks.

# Specification

The complete format specification is defined in rfc.md at the repository root.

## Constants

```go
const (
	// VersionV1 is the current and only supported MDOCX format version.
	VersionV1 uint16 = 1
)
```

Version constants for the MDOCX format.

```go
const (
	// HeaderFlagMetadataJSON indicates that the metadata block contains UTF-8 JSON.
	// This flag MUST be set when metadata is present.
	HeaderFlagMetadataJSON uint16 = 0x0001
)
```

Header flag constants for the fixed header's HeaderFlags field.

## Variables

```go
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
```

Sentinel errors returned by Encode and Decode functions. These errors can be
checked using errors.Is for programmatic error handling.

```go
var Magic = [8]byte{'M', 'D', 'O', 'C', 'X', '\r', '\n', 0x1A}
```

Magic is the 8-byte MDOCX file signature. It consists of "MDOCX\r\n"
followed by 0x1A (SUB character). The magic bytes are designed to detect
common file transfer issues:

- The CR LF sequence detects line-ending conversion
- The 0x1A byte stops display under DOS

## Functions

```go
func Encode(w io.Writer, doc *Document, opts ...WriteOption) error
```

Encode writes doc to w using the MDOCX v1 container format.

The document is validated before writing. Validation includes checking that:

- BundleVersion fields are set to VersionV1
- At least one Markdown file exists
- All paths and IDs are unique and valid
- SHA256 hashes match (if non-zero and verification is enabled)
- Size limits are not exceeded

By default, Encode will:

- Use Zstandard (CompZSTD) compression for both sections
- Auto-populate SHA256 hashes for MediaItems with zero hash (modifies doc in place)
- Verify any non-zero SHA256 hashes match the data

Use WriteOption functions to customize this behavior:

- `WithAutoPopulateSHA256(false)`: don't modify doc
- `WithMarkdownCompression(comp)`: change Markdown section compression
- `WithMediaCompression(comp)`: change Media section compression
- `WithWriteLimits(l)`: set custom size limits
- `WithVerifyHashesOnWrite(false)`: skip hash verification

## Types

```go
type Compression uint16
```

Compression identifies the compression algorithm used for a section payload.

```go
const (
	// CompNone indicates no compression (raw gob bytes).
	CompNone Compression = 0x0
	// CompZIP indicates ZIP container compression (DEFLATE method recommended).
	CompZIP Compression = 0x1
	// CompZSTD indicates Zstandard compression (recommended default).
	CompZSTD Compression = 0x2
	// CompLZ4 indicates LZ4 compression (prioritizes speed over ratio).
	CompLZ4 Compression = 0x3
	// CompBR indicates Brotli compression (prioritizes ratio over speed).
	CompBR Compression = 0x4
)
```

Compression algorithm constants. Writers should prefer CompZSTD as the
default due to its favorable speed/ratio trade-offs.

```go
type Document struct {
	// Metadata contains optional document-level metadata as a JSON-compatible map.
	// If non-nil, it will be serialized as UTF-8 JSON in the file.
	Metadata map[string]any
	// Markdown contains the Markdown files bundle.
	// BundleVersion must be set to VersionV1 and Files must contain at least one entry.
	Markdown MarkdownBundle
	// Media contains the media items bundle.
	// BundleVersion must be set to VersionV1. Items may be empty.
	Media MediaBundle
}
```

Document is the high-level representation of an MDOCX file. It contains
optional metadata, a required Markdown bundle, and an optional Media bundle.

Metadata, if present, is serialized as a UTF-8 JSON object. Common keys include:

- `"title"`: document title
- `"description"`: document description
- `"creator"`: author or creator name
- `"created_at"`: RFC3339 timestamp
- `"root"`: path to the primary Markdown file
- `"tags"`: array of string tags

```go
func Decode(r io.Reader, opts ...ReadOption) (*Document, error)
```

Decode reads an MDOCX document from r.

The decoding process:

1. Reads and validates the 32-byte fixed header
2. Reads and parses the optional metadata block as JSON
3. Reads and decompresses the Markdown bundle section
4. Reads and decompresses the Media bundle section
5. Validates the complete document

By default, Decode will:

- Use safe default size limits (see DefaultLimits)
- Verify SHA256 hashes if present

Use ReadOption functions to customize this behavior:

- `WithReadLimits(l)`: set custom size limits
- `WithVerifyHashes(false)`: skip hash verification

Decode returns ErrInvalidMagic if the file is not an MDOCX file,
ErrUnsupportedVersion if the version is not 1, ErrLimitExceeded if any size
limit is exceeded, or ErrValidation if the document fails validation.

```go
type Limits struct {
	// MaxMetadataLen is the maximum allowed length of the metadata JSON block in bytes.
	MaxMetadataLen uint32
	// MaxMarkdownSectionLen is the maximum compressed payload length for the Markdown section.
	MaxMarkdownSectionLen uint64
	// MaxMediaSectionLen is the maximum compressed payload length for the Media section.
	MaxMediaSectionLen uint64
	// MaxMarkdownUncompressed is the maximum decompressed size of the Markdown gob payload.
	MaxMarkdownUncompressed uint64
	// MaxMediaUncompressed is the maximum decompressed size of the Media gob payload.
	MaxMediaUncompressed uint64
	// MaxMarkdownFiles is the maximum number of Markdown files allowed in the bundle.
	MaxMarkdownFiles int
	// MaxMediaItems is the maximum number of media items allowed in the bundle.
	MaxMediaItems int
	// MaxSingleMarkdownFileSize is the maximum size of a single Markdown file's content.
	MaxSingleMarkdownFileSize uint64
	// MaxSingleMediaSize is the maximum size of a single media item's data.
	MaxSingleMediaSize uint64
}
```

Limits defines size and count limits enforced during encoding and decoding.
These limits protect against resource exhaustion from malformed or malicious
input.

Zero values for any field will be replaced with safe defaults when used.
To disable a limit, set it to a very large value (not zero).

Default limits are based on the MDOCX specification recommendations:

- MaxMetadataLen: 1 MiB
- MaxMarkdownSectionLen: 1 GiB (compressed payload)
- MaxMediaSectionLen: 4 GiB (compressed payload)
- MaxMarkdownUncompressed: 256 MiB
- MaxMediaUncompressed: 2 GiB
- MaxMarkdownFiles: 10,000
- MaxMediaItems: 10,000
- MaxSingleMarkdownFileSize: 256 MiB
- MaxSingleMediaSize: 512 MiB

```go
func DefaultLimits() Limits
```

DefaultLimits returns the default size limits as recommended by the
MDOCX specification. These defaults provide a reasonable balance between
flexibility and security.

```go
type MarkdownBundle struct {
	// BundleVersion must be VersionV1 (1) for this specification.
	BundleVersion uint16
	// RootPath optionally specifies the primary Markdown file path.
	// If non-empty, it overrides metadata.root.
	RootPath string
	// Files contains the Markdown files in the bundle.
	// Must contain at least one entry.
	Files []MarkdownFile
}
```

MarkdownBundle contains one or more Markdown files. It is serialized using
gob encoding in section 1 of an MDOCX file.

```go
type MarkdownFile struct {
	// Path is the container path (e.g., "docs/readme.md").
	// Must be unique within the bundle, use forward slashes, and not be absolute.
	Path string
	// Content is the UTF-8 encoded Markdown content.
	Content []byte
	// MediaRefs optionally lists IDs of referenced media items.
	MediaRefs []string
	// Attributes holds arbitrary per-file metadata as key-value pairs.
	Attributes map[string]string
}
```

MarkdownFile represents a single Markdown document within a bundle.

```go
type MediaBundle struct {
	// BundleVersion must be VersionV1 (1) for this specification.
	BundleVersion uint16
	// Items contains the media items in the bundle.
	// May be empty.
	Items []MediaItem
}
```

MediaBundle contains zero or more media items. It is serialized using gob
encoding in section 2 of an MDOCX file.

```go
type MediaItem struct {
	// ID is a stable unique identifier for this media item.
	// Used for mdocx://media/<ID> URI references.
	ID string
	// Path optionally specifies a container path (e.g., "assets/logo.png").
	// Used for relative path references from Markdown.
	Path string
	// MIMEType specifies the media type (e.g., "image/png", "audio/mpeg").
	MIMEType string
	// Data contains the raw binary content of the media item.
	Data []byte
	// SHA256 optionally contains the SHA-256 hash of Data for integrity verification.
	// If non-zero, it must match the computed hash of Data.
	SHA256 [32]byte
	// Attributes holds arbitrary per-item metadata as key-value pairs.
	Attributes map[string]string
}
```

MediaItem represents a single media asset (image, audio, video, etc.).

```go
type ReadOption func(*readConfig)
```

ReadOption is a functional option for configuring Decode behavior.

```go
func WithReadLimits(l Limits) ReadOption
```

WithReadLimits sets custom size limits for decoding. Zero values in l will
be replaced with safe defaults.

Example:

```go
doc, err := mdocx.Decode(r, mdocx.WithReadLimits(mdocx.Limits{
	MaxMediaUncompressed: 4 << 30, // Allow up to 4 GiB
}))
```

```go
func WithVerifyHashes(v bool) ReadOption
```

WithVerifyHashes controls whether non-zero MediaItem.SHA256 fields are
verified during decode. When enabled (default), any SHA256 mismatch will
cause Decode to return ErrValidation. Disable this for faster decoding when
integrity has been verified externally.

```go
type SectionType uint16
```

SectionType identifies the type of a section in an MDOCX file.

```go
const (
	// SectionMarkdown identifies the Markdown bundle section (must appear first).
	SectionMarkdown SectionType = 1
	// SectionMedia identifies the Media bundle section (must appear second).
	SectionMedia SectionType = 2
)
```

Section type constants.

```go
type WriteOption func(*writeConfig)
```

WriteOption is a functional option for configuring Encode behavior.

```go
func WithAutoPopulateSHA256(v bool) WriteOption
```

WithAutoPopulateSHA256 controls whether Encode automatically computes SHA256
hashes for MediaItems that have a zero hash value. When enabled (default),
doc.Media.Items will be modified in place to add computed hashes. Disable
this if you need the document to remain unmodified.

```go
func WithMarkdownCompression(comp Compression) WriteOption
```

WithMarkdownCompression sets the compression algorithm for the Markdown
section. Default is CompZSTD. Use CompNone to disable compression.

Compression selection guidance:

- CompZSTD: Recommended default, good speed/ratio balance
- CompZIP: Maximum interoperability with other tools
- CompLZ4: Maximum encode/decode speed
- CompBR: Maximum compression ratio (slower)

```go
func WithMediaCompression(comp Compression) WriteOption
```

WithMediaCompression sets the compression algorithm for the Media section.
Default is CompZSTD. Use CompNone to disable compression. Note that media
files (images, video) are often already compressed, so compression may not
provide significant size reduction.

```go
func WithVerifyHashesOnWrite(v bool) WriteOption
```

WithVerifyHashesOnWrite controls whether non-zero MediaItem.SHA256 fields
are verified during encode. When enabled (default), any SHA256 mismatch will
cause Encode to return ErrValidation. This verifies that provided hashes
match the actual data before writing.

```go
func WithWriteLimits(l Limits) WriteOption
```

WithWriteLimits sets custom size limits for encoding. Zero values in l will
be replaced with safe defaults. Encoding will fail with ErrLimitExceeded if
any limit is exceeded.

