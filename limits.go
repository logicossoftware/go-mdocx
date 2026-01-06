package mdocx

// Limits defines size and count limits enforced during encoding and decoding.
// These limits protect against resource exhaustion from malformed or malicious input.
//
// Zero values for any field will be replaced with safe defaults when used.
// To disable a limit, set it to a very large value (not zero).
//
// Default limits are based on the MDOCX specification recommendations:
//   - MaxMetadataLen: 1 MiB
//   - MaxMarkdownSectionLen: 1 GiB (compressed payload)
//   - MaxMediaSectionLen: 4 GiB (compressed payload)
//   - MaxMarkdownUncompressed: 256 MiB
//   - MaxMediaUncompressed: 2 GiB
//   - MaxMarkdownFiles: 10,000
//   - MaxMediaItems: 10,000
//   - MaxSingleMarkdownFileSize: 256 MiB
//   - MaxSingleMediaSize: 512 MiB
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

// DefaultLimits returns the default size limits as recommended by the MDOCX specification.
// These defaults provide a reasonable balance between flexibility and security.
func DefaultLimits() Limits {
	return defaultLimits()
}

// defaultLimits returns the internal default limits configuration.
func defaultLimits() Limits {
	return Limits{
		MaxMetadataLen:            1 << 20,   // 1 MiB
		MaxMarkdownSectionLen:     1 << 30,   // 1 GiB stored payload cap
		MaxMediaSectionLen:        1 << 32,   // 4 GiB stored payload cap
		MaxMarkdownUncompressed:   256 << 20, // 256 MiB
		MaxMediaUncompressed:      2 << 30,   // 2 GiB
		MaxMarkdownFiles:          10_000,
		MaxMediaItems:             10_000,
		MaxSingleMarkdownFileSize: 256 << 20,
		MaxSingleMediaSize:        512 << 20,
	}
}

// withDefaults returns a copy of l with zero fields replaced by default values.
func (l Limits) withDefaults() Limits {
	d := defaultLimits()
	if l.MaxMetadataLen == 0 {
		l.MaxMetadataLen = d.MaxMetadataLen
	}
	if l.MaxMarkdownSectionLen == 0 {
		l.MaxMarkdownSectionLen = d.MaxMarkdownSectionLen
	}
	if l.MaxMediaSectionLen == 0 {
		l.MaxMediaSectionLen = d.MaxMediaSectionLen
	}
	if l.MaxMarkdownUncompressed == 0 {
		l.MaxMarkdownUncompressed = d.MaxMarkdownUncompressed
	}
	if l.MaxMediaUncompressed == 0 {
		l.MaxMediaUncompressed = d.MaxMediaUncompressed
	}
	if l.MaxMarkdownFiles == 0 {
		l.MaxMarkdownFiles = d.MaxMarkdownFiles
	}
	if l.MaxMediaItems == 0 {
		l.MaxMediaItems = d.MaxMediaItems
	}
	if l.MaxSingleMarkdownFileSize == 0 {
		l.MaxSingleMarkdownFileSize = d.MaxSingleMarkdownFileSize
	}
	if l.MaxSingleMediaSize == 0 {
		l.MaxSingleMediaSize = d.MaxSingleMediaSize
	}
	return l
}
