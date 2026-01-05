package mdocx

import "crypto/sha256"

const (
	VersionV1 uint16 = 1

	fixedHeaderSizeV1 uint32 = 32
)

// Magic is the 8-byte MDOCX file signature.
var Magic = [8]byte{'M', 'D', 'O', 'C', 'X', '\r', '\n', 0x1A}

const (
	HeaderFlagMetadataJSON uint16 = 0x0001
)

type SectionType uint16

const (
	SectionMarkdown SectionType = 1
	SectionMedia    SectionType = 2
)

type Compression uint16

const (
	CompNone Compression = 0x0
	CompZIP  Compression = 0x1
	CompZSTD Compression = 0x2
	CompLZ4  Compression = 0x3
	CompBR   Compression = 0x4
)

const (
	sectionFlagCompressionMask    uint16 = 0x000F
	sectionFlagHasUncompressedLen uint16 = 0x0010
)

type MarkdownBundle struct {
	BundleVersion uint16
	RootPath      string
	Files         []MarkdownFile
}

type MarkdownFile struct {
	Path       string
	Content    []byte
	MediaRefs  []string
	Attributes map[string]string
}

type MediaBundle struct {
	BundleVersion uint16
	Items         []MediaItem
}

type MediaItem struct {
	ID         string
	Path       string
	MIMEType   string
	Data       []byte
	SHA256     [32]byte
	Attributes map[string]string
}

func (m MediaItem) computedSHA256() [32]byte {
	return sha256.Sum256(m.Data)
}

// Document is a logical representation of an MDOCX file.
//
// Metadata is optional and, if present, is encoded as a JSON object.
// Markdown MUST be present.
// Media MAY be empty.
type Document struct {
	Metadata map[string]any
	Markdown MarkdownBundle
	Media    MediaBundle
}
