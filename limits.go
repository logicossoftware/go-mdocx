package mdocx

type Limits struct {
	MaxMetadataLen            uint32
	MaxMarkdownSectionLen     uint64 // compressed payload length as stored in file
	MaxMediaSectionLen        uint64 // compressed payload length as stored in file
	MaxMarkdownUncompressed   uint64 // gob bytes after decompression
	MaxMediaUncompressed      uint64 // gob bytes after decompression
	MaxMarkdownFiles          int
	MaxMediaItems             int
	MaxSingleMarkdownFileSize uint64
	MaxSingleMediaSize        uint64
}

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
