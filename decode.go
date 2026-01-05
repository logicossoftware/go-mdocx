package mdocx

import (
	"bytes"
	"encoding/gob"
	"encoding/json"
	"fmt"
	"io"
)

// Decode reads an MDOCX document from r.
func Decode(r io.Reader, opts ...ReadOption) (*Document, error) {
	cfg := readConfig{limits: defaultLimits(), verifyHashes: true}
	for _, opt := range opts {
		opt(&cfg)
	}
	cfg.limits = cfg.limits.withDefaults()

	h, err := readFixedHeader(r)
	if err != nil {
		return nil, err
	}
	if h.Magic != Magic {
		return nil, ErrInvalidMagic
	}
	if h.FixedHdrSize != fixedHeaderSizeV1 {
		return nil, fmt.Errorf("%w: fixed header size %d", ErrInvalidHeader, h.FixedHdrSize)
	}
	if h.Version != VersionV1 {
		return nil, ErrUnsupportedVersion
	}
	if h.Reserved0 != 0 || h.Reserved1 != 0 {
		return nil, fmt.Errorf("%w: reserved must be zero", ErrInvalidHeader)
	}
	if h.MetadataLength > cfg.limits.MaxMetadataLen {
		return nil, fmt.Errorf("%w: metadata length %d", ErrLimitExceeded, h.MetadataLength)
	}

	var metadata map[string]any
	if h.MetadataLength > 0 {
		mb := make([]byte, h.MetadataLength)
		if _, err := io.ReadFull(r, mb); err != nil {
			return nil, err
		}
		if (h.HeaderFlags & HeaderFlagMetadataJSON) == 0 {
			return nil, fmt.Errorf("%w: metadata present but METADATA_JSON flag not set", ErrInvalidHeader)
		}
		if err := json.Unmarshal(mb, &metadata); err != nil {
			return nil, err
		}
		if metadata == nil {
			return nil, fmt.Errorf("%w: metadata must be a JSON object", ErrInvalidHeader)
		}
	}

	mdSec, err := readSectionHeader(r)
	if err != nil {
		return nil, err
	}
	if err := validateSectionHeader(mdSec, SectionMarkdown); err != nil {
		return nil, err
	}
	if mdSec.PayloadLen > cfg.limits.MaxMarkdownSectionLen {
		return nil, fmt.Errorf("%w: markdown section too large", ErrLimitExceeded)
	}
	mdPayload := make([]byte, mdSec.PayloadLen)
	if _, err := io.ReadFull(r, mdPayload); err != nil {
		return nil, err
	}
	mdGob, err := decompressPayload(mdSec.compression(), mdSec.SectionFlags, mdPayload, cfg.limits.MaxMarkdownUncompressed)
	if err != nil {
		return nil, err
	}
	var markdown MarkdownBundle
	if err := gobDecode(mdGob, &markdown); err != nil {
		return nil, err
	}

	mediaSec, err := readSectionHeader(r)
	if err != nil {
		return nil, err
	}
	if err := validateSectionHeader(mediaSec, SectionMedia); err != nil {
		return nil, err
	}
	if mediaSec.PayloadLen > cfg.limits.MaxMediaSectionLen {
		return nil, fmt.Errorf("%w: media section too large", ErrLimitExceeded)
	}
	var media MediaBundle
	if mediaSec.PayloadLen == 0 {
		media = MediaBundle{BundleVersion: VersionV1}
	} else {
		mediaPayload := make([]byte, mediaSec.PayloadLen)
		if _, err := io.ReadFull(r, mediaPayload); err != nil {
			return nil, err
		}
		mediaGob, err := decompressPayload(mediaSec.compression(), mediaSec.SectionFlags, mediaPayload, cfg.limits.MaxMediaUncompressed)
		if err != nil {
			return nil, err
		}
		if err := gobDecode(mediaGob, &media); err != nil {
			return nil, err
		}
	}

	doc := &Document{Metadata: metadata, Markdown: markdown, Media: media}
	if err := validateDocument(doc, cfg.limits, cfg.verifyHashes); err != nil {
		return nil, err
	}
	return doc, nil
}

func gobDecode(data []byte, out any) error {
	dec := gob.NewDecoder(bytes.NewReader(data))
	return dec.Decode(out)
}
