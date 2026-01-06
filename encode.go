package mdocx

import (
	"bytes"
	"encoding/gob"
	"encoding/json"
	"fmt"
	"io"
)

// Function variables for testing injection.
var (
	gobEncodeMarkdown = func(v MarkdownBundle) ([]byte, error) { return gobEncode(v) }
	gobEncodeMedia    = func(v MediaBundle) ([]byte, error) { return gobEncode(v) }
)

// Encode writes doc to w using the MDOCX v1 container format.
//
// The document is validated before writing. Validation includes checking that:
//   - BundleVersion fields are set to VersionV1
//   - At least one Markdown file exists
//   - All paths and IDs are unique and valid
//   - SHA256 hashes match (if non-zero and verification is enabled)
//   - Size limits are not exceeded
//
// By default, Encode will:
//   - Use Zstandard (CompZSTD) compression for both sections
//   - Auto-populate SHA256 hashes for MediaItems with zero hash (modifies doc in place)
//   - Verify any non-zero SHA256 hashes match the data
//
// Use WriteOption functions to customize this behavior:
//   - WithAutoPopulateSHA256(false): don't modify doc
//   - WithMarkdownCompression(comp): change Markdown section compression
//   - WithMediaCompression(comp): change Media section compression
//   - WithWriteLimits(l): set custom size limits
//   - WithVerifyHashesOnWrite(false): skip hash verification
func Encode(w io.Writer, doc *Document, opts ...WriteOption) error {
	cfg := writeConfig{
		limits:           defaultLimits(),
		verifyHashes:     true,
		autoPopulate:     true,
		mdCompression:    CompZSTD,
		mediaCompression: CompZSTD,
	}
	for _, opt := range opts {
		opt(&cfg)
	}
	cfg.limits = cfg.limits.withDefaults()
	if doc == nil {
		return fmt.Errorf("%w: document is nil", ErrValidation)
	}

	if cfg.autoPopulate {
		for i := range doc.Media.Items {
			if doc.Media.Items[i].SHA256 == ([32]byte{}) {
				doc.Media.Items[i].SHA256 = doc.Media.Items[i].computedSHA256()
			}
		}
	}

	if err := validateDocument(doc, cfg.limits, cfg.verifyHashes); err != nil {
		return err
	}

	var metadataBytes []byte
	var headerFlags uint16
	if doc.Metadata != nil {
		b, err := json.Marshal(doc.Metadata)
		if err != nil {
			return err
		}
		if len(b) > int(cfg.limits.MaxMetadataLen) {
			return fmt.Errorf("%w: metadata too large", ErrLimitExceeded)
		}
		metadataBytes = b
		headerFlags |= HeaderFlagMetadataJSON
	}

	mdGob, err := gobEncodeMarkdown(doc.Markdown)
	if err != nil {
		return err
	}
	mediaGob, err := gobEncodeMedia(doc.Media)
	if err != nil {
		return err
	}

	mdFlags, mdPayload, err := compressPayload(cfg.mdCompression, mdGob)
	if err != nil {
		return err
	}
	mediaFlags, mediaPayload, err := compressPayload(cfg.mediaCompression, mediaGob)
	if err != nil {
		return err
	}

	h := fixedHeaderV1{
		Magic:          Magic,
		Version:        VersionV1,
		HeaderFlags:    headerFlags,
		FixedHdrSize:   fixedHeaderSizeV1,
		MetadataLength: uint32(len(metadataBytes)),
		Reserved0:      0,
		Reserved1:      0,
	}
	if err := writeFixedHeader(w, h); err != nil {
		return err
	}
	if len(metadataBytes) > 0 {
		if _, err := w.Write(metadataBytes); err != nil {
			return err
		}
	}

	mdHeader := sectionHeaderV1{
		SectionType:  uint16(SectionMarkdown),
		SectionFlags: mdFlags,
		PayloadLen:   uint64(len(mdPayload)),
		Reserved:     0,
	}
	if err := writeSectionHeader(w, mdHeader); err != nil {
		return err
	}
	if _, err := w.Write(mdPayload); err != nil {
		return err
	}

	mediaHeader := sectionHeaderV1{
		SectionType:  uint16(SectionMedia),
		SectionFlags: mediaFlags,
		PayloadLen:   uint64(len(mediaPayload)),
		Reserved:     0,
	}
	if err := writeSectionHeader(w, mediaHeader); err != nil {
		return err
	}
	_, err = w.Write(mediaPayload)
	return err
}

// gobEncode serializes v using Go's gob encoding.
func gobEncode[T any](v T) ([]byte, error) {
	var buf bytes.Buffer
	enc := gob.NewEncoder(&buf)
	if err := enc.Encode(v); err != nil {
		return nil, err
	}
	return buf.Bytes(), nil
}
