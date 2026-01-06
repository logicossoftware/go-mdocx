package mdocx

import "crypto/sha256"

// Version constants for the MDOCX format.
const (
	// VersionV1 is the current and only supported MDOCX format version.
	VersionV1 uint16 = 1

	// fixedHeaderSizeV1 is the size in bytes of the fixed header for v1 files.
	fixedHeaderSizeV1 uint32 = 32
)

// Magic is the 8-byte MDOCX file signature.
// It consists of "MDOCX\r\n" followed by 0x1A (SUB character).
// The magic bytes are designed to detect common file transfer issues:
//   - The CR LF sequence detects line-ending conversion
//   - The 0x1A byte stops display under DOS
var Magic = [8]byte{'M', 'D', 'O', 'C', 'X', '\r', '\n', 0x1A}

// Header flag constants for the fixed header's HeaderFlags field.
const (
	// HeaderFlagMetadataJSON indicates that the metadata block contains UTF-8 JSON.
	// This flag MUST be set when metadata is present.
	HeaderFlagMetadataJSON uint16 = 0x0001
)

// SectionType identifies the type of a section in an MDOCX file.
type SectionType uint16

// Section type constants.
const (
	// SectionMarkdown identifies the Markdown bundle section (must appear first).
	SectionMarkdown SectionType = 1
	// SectionMedia identifies the Media bundle section (must appear second).
	SectionMedia SectionType = 2
)

// Compression identifies the compression algorithm used for a section payload.
type Compression uint16

// Compression algorithm constants.
// Writers should prefer CompZSTD as the default due to its favorable speed/ratio trade-offs.
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

// Internal section flag masks.
const (
	// sectionFlagCompressionMask extracts the compression algorithm from SectionFlags (bits 0-3).
	sectionFlagCompressionMask uint16 = 0x000F
	// sectionFlagHasUncompressedLen indicates the payload has an 8-byte uncompressed length prefix.
	sectionFlagHasUncompressedLen uint16 = 0x0010
)

// MarkdownBundle contains one or more Markdown files.
// It is serialized using gob encoding in section 1 of an MDOCX file.
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

// MarkdownFile represents a single Markdown document within a bundle.
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

// MediaBundle contains zero or more media items.
// It is serialized using gob encoding in section 2 of an MDOCX file.
type MediaBundle struct {
	// BundleVersion must be VersionV1 (1) for this specification.
	BundleVersion uint16
	// Items contains the media items in the bundle.
	// May be empty.
	Items []MediaItem
}

// MediaItem represents a single media asset (image, audio, video, etc.).
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

// computedSHA256 returns the SHA-256 hash of the media item's data.
func (m MediaItem) computedSHA256() [32]byte {
	return sha256.Sum256(m.Data)
}

// Document is the high-level representation of an MDOCX file.
// It contains optional metadata, a required Markdown bundle, and an optional Media bundle.
//
// Metadata, if present, is serialized as a UTF-8 JSON object. Common keys include:
//   - "title": document title
//   - "description": document description
//   - "creator": author or creator name
//   - "created_at": RFC3339 timestamp
//   - "root": path to the primary Markdown file
//   - "tags": array of string tags
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
