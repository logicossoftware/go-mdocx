package mdocx

import (
	"bytes"
	"encoding/binary"
	"errors"
	"io"
	"reflect"
	"testing"
)

func sampleDoc() *Document {
	return &Document{
		Metadata: map[string]any{
			"title": "Example",
			"tags":  []any{"a", "b"},
		},
		Markdown: MarkdownBundle{
			BundleVersion: VersionV1,
			RootPath:      "docs/index.md",
			Files: []MarkdownFile{
				{Path: "docs/index.md", Content: []byte("# Hello\n\n![Logo](mdocx://media/logo)\n"), MediaRefs: []string{"logo"}},
				{Path: "docs/notes.md", Content: []byte("Some notes\n")},
			},
		},
		Media: MediaBundle{
			BundleVersion: VersionV1,
			Items: []MediaItem{
				{ID: "logo", Path: "assets/logo.png", MIMEType: "image/png", Data: []byte{0x01, 0x02, 0x03}}, // SHA auto-populated
			},
		},
	}
}

type failingWriter struct {
	n int
}

func (w *failingWriter) Write(p []byte) (int, error) {
	if w.n <= 0 {
		return 0, io.ErrClosedPipe
	}
	if len(p) > w.n {
		p = p[:w.n]
	}
	w.n -= len(p)
	return len(p), nil
}

func TestWireRoundtrip(t *testing.T) {
	in := fixedHeaderV1{Magic: Magic, Version: VersionV1, HeaderFlags: HeaderFlagMetadataJSON, FixedHdrSize: fixedHeaderSizeV1, MetadataLength: 123}
	var buf bytes.Buffer
	if err := writeFixedHeader(&buf, in); err != nil {
		t.Fatal(err)
	}
	out, err := readFixedHeader(&buf)
	if err != nil {
		t.Fatal(err)
	}
	if !reflect.DeepEqual(in, out) {
		t.Fatalf("fixed header mismatch: %#v vs %#v", in, out)
	}

	buf.Reset()
	shIn := sectionHeaderV1{SectionType: uint16(SectionMarkdown), SectionFlags: uint16(CompNone), PayloadLen: 99, Reserved: 0}
	if err := writeSectionHeader(&buf, shIn); err != nil {
		t.Fatal(err)
	}
	shOut, err := readSectionHeader(&buf)
	if err != nil {
		t.Fatal(err)
	}
	if !reflect.DeepEqual(shIn, shOut) {
		t.Fatalf("section header mismatch: %#v vs %#v", shIn, shOut)
	}
}

func TestEncodeDecodeRoundTrip_AllCompressions(t *testing.T) {
	comps := []Compression{CompNone, CompZIP, CompZSTD, CompLZ4, CompBR}
	for _, comp := range comps {
		t.Run("comp="+compressionName(comp), func(t *testing.T) {
			doc := sampleDoc()
			var buf bytes.Buffer
			if err := Encode(&buf, doc, WithMarkdownCompression(comp), WithMediaCompression(comp)); err != nil {
				t.Fatalf("Encode: %v", err)
			}
			got, err := Decode(bytes.NewReader(buf.Bytes()))
			if err != nil {
				t.Fatalf("Decode: %v", err)
			}
			// Encode auto-populates SHA256; compare against the mutated input doc.
			if !reflect.DeepEqual(doc, got) {
				t.Fatalf("doc mismatch\nwant: %#v\ngot:  %#v", doc, got)
			}
		})
	}
}

func TestEncodeNilDocument(t *testing.T) {
	var buf bytes.Buffer
	err := Encode(&buf, nil)
	if err == nil {
		t.Fatal("expected error")
	}
	if !errors.Is(err, ErrValidation) {
		t.Fatalf("expected ErrValidation, got %v", err)
	}
}

func TestEncodeWriterError(t *testing.T) {
	doc := sampleDoc()
	w := &failingWriter{n: 10}
	err := Encode(w, doc, WithMarkdownCompression(CompNone), WithMediaCompression(CompNone))
	if err == nil {
		t.Fatal("expected error")
	}
}

func TestDecode_InvalidMagic(t *testing.T) {
	doc := sampleDoc()
	var buf bytes.Buffer
	if err := Encode(&buf, doc, WithMarkdownCompression(CompNone), WithMediaCompression(CompNone)); err != nil {
		t.Fatal(err)
	}
	b := buf.Bytes()
	b[0] ^= 0xFF
	_, err := Decode(bytes.NewReader(b))
	if !errors.Is(err, ErrInvalidMagic) {
		t.Fatalf("expected ErrInvalidMagic, got %v", err)
	}
}

func TestDecode_InvalidFixedHeaderSize(t *testing.T) {
	doc := sampleDoc()
	var buf bytes.Buffer
	if err := Encode(&buf, doc); err != nil {
		t.Fatal(err)
	}
	b := buf.Bytes()
	binary.LittleEndian.PutUint32(b[12:16], 31)
	_, err := Decode(bytes.NewReader(b))
	if err == nil || !errors.Is(err, ErrInvalidHeader) {
		t.Fatalf("expected ErrInvalidHeader, got %v", err)
	}
}

func TestDecode_UnsupportedVersion(t *testing.T) {
	doc := sampleDoc()
	var buf bytes.Buffer
	if err := Encode(&buf, doc); err != nil {
		t.Fatal(err)
	}
	b := buf.Bytes()
	binary.LittleEndian.PutUint16(b[8:10], 2)
	_, err := Decode(bytes.NewReader(b))
	if !errors.Is(err, ErrUnsupportedVersion) {
		t.Fatalf("expected ErrUnsupportedVersion, got %v", err)
	}
}

func TestDecode_ReservedNonZero(t *testing.T) {
	doc := sampleDoc()
	var buf bytes.Buffer
	if err := Encode(&buf, doc); err != nil {
		t.Fatal(err)
	}
	b := buf.Bytes()
	binary.LittleEndian.PutUint32(b[20:24], 1)
	_, err := Decode(bytes.NewReader(b))
	if err == nil || !errors.Is(err, ErrInvalidHeader) {
		t.Fatalf("expected ErrInvalidHeader, got %v", err)
	}
}

func TestDecode_MetadataPresentButFlagMissing(t *testing.T) {
	doc := sampleDoc()
	var buf bytes.Buffer
	if err := Encode(&buf, doc, WithMarkdownCompression(CompNone), WithMediaCompression(CompNone)); err != nil {
		t.Fatal(err)
	}
	b := buf.Bytes()
	binary.LittleEndian.PutUint16(b[10:12], 0)
	_, err := Decode(bytes.NewReader(b))
	if err == nil || !errors.Is(err, ErrInvalidHeader) {
		t.Fatalf("expected ErrInvalidHeader, got %v", err)
	}
}

func TestDecode_MetadataInvalidJSON(t *testing.T) {
	// Build minimal container with invalid JSON metadata.
	var buf bytes.Buffer
	h := fixedHeaderV1{Magic: Magic, Version: VersionV1, HeaderFlags: HeaderFlagMetadataJSON, FixedHdrSize: fixedHeaderSizeV1, MetadataLength: 2}
	if err := writeFixedHeader(&buf, h); err != nil {
		t.Fatal(err)
	}
	buf.WriteString("[]")
	mdHeader := sectionHeaderV1{SectionType: uint16(SectionMarkdown), SectionFlags: uint16(CompNone), PayloadLen: 0, Reserved: 0}
	if err := writeSectionHeader(&buf, mdHeader); err != nil {
		t.Fatal(err)
	}
	mediaHeader := sectionHeaderV1{SectionType: uint16(SectionMedia), SectionFlags: uint16(CompNone), PayloadLen: 0, Reserved: 0}
	if err := writeSectionHeader(&buf, mediaHeader); err != nil {
		t.Fatal(err)
	}
	_, err := Decode(bytes.NewReader(buf.Bytes()))
	if err == nil {
		t.Fatal("expected error")
	}
}

func TestDecode_SectionTypeMismatch(t *testing.T) {
	doc := sampleDoc()
	var buf bytes.Buffer
	if err := Encode(&buf, doc, WithMarkdownCompression(CompNone), WithMediaCompression(CompNone)); err != nil {
		t.Fatal(err)
	}
	b := buf.Bytes()
	// First section header starts at fixed header (32) + metadata length.
	off := 32 + int(binary.LittleEndian.Uint32(b[16:20]))
	binary.LittleEndian.PutUint16(b[off:off+2], uint16(SectionMedia))
	_, err := Decode(bytes.NewReader(b))
	if err == nil || !errors.Is(err, ErrInvalidSection) {
		t.Fatalf("expected ErrInvalidSection, got %v", err)
	}
}

func TestDecode_SectionReservedNonZero(t *testing.T) {
	doc := sampleDoc()
	var buf bytes.Buffer
	if err := Encode(&buf, doc, WithMarkdownCompression(CompNone), WithMediaCompression(CompNone)); err != nil {
		t.Fatal(err)
	}
	b := buf.Bytes()
	off := 32 + int(binary.LittleEndian.Uint32(b[16:20]))
	binary.LittleEndian.PutUint32(b[off+12:off+16], 1)
	_, err := Decode(bytes.NewReader(b))
	if err == nil || !errors.Is(err, ErrInvalidSection) {
		t.Fatalf("expected ErrInvalidSection, got %v", err)
	}
}

func TestDecode_SectionFlagsInvalid_NoneWithLen(t *testing.T) {
	doc := sampleDoc()
	var buf bytes.Buffer
	if err := Encode(&buf, doc, WithMarkdownCompression(CompNone), WithMediaCompression(CompNone)); err != nil {
		t.Fatal(err)
	}
	b := buf.Bytes()
	off := 32 + int(binary.LittleEndian.Uint32(b[16:20]))
	binary.LittleEndian.PutUint16(b[off+2:off+4], sectionFlagHasUncompressedLen) // COMP_NONE + HAS
	_, err := Decode(bytes.NewReader(b))
	if err == nil || !errors.Is(err, ErrInvalidSection) {
		t.Fatalf("expected ErrInvalidSection, got %v", err)
	}
}

func TestDecode_SectionFlagsInvalid_CompressedMissingLen(t *testing.T) {
	doc := sampleDoc()
	var buf bytes.Buffer
	if err := Encode(&buf, doc, WithMarkdownCompression(CompZSTD), WithMediaCompression(CompNone)); err != nil {
		t.Fatal(err)
	}
	b := buf.Bytes()
	off := 32 + int(binary.LittleEndian.Uint32(b[16:20]))
	binary.LittleEndian.PutUint16(b[off+2:off+4], uint16(CompZSTD)) // compressed but missing HAS
	_, err := Decode(bytes.NewReader(b))
	if err == nil || !errors.Is(err, ErrInvalidSection) {
		t.Fatalf("expected ErrInvalidSection, got %v", err)
	}
}

func TestDecode_UnknownCompression(t *testing.T) {
	doc := sampleDoc()
	var buf bytes.Buffer
	if err := Encode(&buf, doc, WithMarkdownCompression(CompNone), WithMediaCompression(CompNone)); err != nil {
		t.Fatal(err)
	}
	b := buf.Bytes()
	off := 32 + int(binary.LittleEndian.Uint32(b[16:20]))
	binary.LittleEndian.PutUint16(b[off+2:off+4], 0x000F) // unknown compression value
	_, err := Decode(bytes.NewReader(b))
	if err == nil || !errors.Is(err, ErrInvalidSection) {
		t.Fatalf("expected ErrInvalidSection, got %v", err)
	}
}

func TestDecode_MetadataLimitExceeded(t *testing.T) {
	doc := sampleDoc()
	var buf bytes.Buffer
	if err := Encode(&buf, doc, WithMarkdownCompression(CompNone), WithMediaCompression(CompNone)); err != nil {
		t.Fatal(err)
	}
	_, err := Decode(bytes.NewReader(buf.Bytes()), WithReadLimits(Limits{MaxMetadataLen: 1}))
	if err == nil || !errors.Is(err, ErrLimitExceeded) {
		t.Fatalf("expected ErrLimitExceeded, got %v", err)
	}
}

func TestDecode_SectionStoredLengthLimitExceeded(t *testing.T) {
	// Minimal valid header + empty metadata + markdown section header with huge PayloadLen.
	var buf bytes.Buffer
	h := fixedHeaderV1{Magic: Magic, Version: VersionV1, FixedHdrSize: fixedHeaderSizeV1}
	if err := writeFixedHeader(&buf, h); err != nil {
		t.Fatal(err)
	}
	mdHeader := sectionHeaderV1{SectionType: uint16(SectionMarkdown), SectionFlags: uint16(CompNone), PayloadLen: 9999, Reserved: 0}
	if err := writeSectionHeader(&buf, mdHeader); err != nil {
		t.Fatal(err)
	}
	_, err := Decode(bytes.NewReader(buf.Bytes()), WithReadLimits(Limits{MaxMarkdownSectionLen: 1}))
	if err == nil || !errors.Is(err, ErrLimitExceeded) {
		t.Fatalf("expected ErrLimitExceeded, got %v", err)
	}
}

func TestEncode_MetadataTooLarge(t *testing.T) {
	doc := sampleDoc()
	var buf bytes.Buffer
	err := Encode(&buf, doc, WithWriteLimits(Limits{MaxMetadataLen: 1}), WithMarkdownCompression(CompNone), WithMediaCompression(CompNone))
	if err == nil || !errors.Is(err, ErrLimitExceeded) {
		t.Fatalf("expected ErrLimitExceeded, got %v", err)
	}
}

func TestDecode_EmptyMediaPayloadLenZero(t *testing.T) {
	doc := sampleDoc()
	doc.Media.Items = nil
	var buf bytes.Buffer
	if err := Encode(&buf, doc, WithMarkdownCompression(CompZSTD), WithMediaCompression(CompZSTD)); err != nil {
		t.Fatal(err)
	}
	b := buf.Bytes()
	metaLen := int(binary.LittleEndian.Uint32(b[16:20]))
	mdOff := 32 + metaLen
	mdPayloadLen := int(binary.LittleEndian.Uint64(b[mdOff+4 : mdOff+12]))
	mediaOff := mdOff + 16 + mdPayloadLen
	// Set media PayloadLen to 0 and truncate the payload bytes.
	binary.LittleEndian.PutUint64(b[mediaOff+4:mediaOff+12], 0)
	b = b[:mediaOff+16]

	got, err := Decode(bytes.NewReader(b))
	if err != nil {
		t.Fatal(err)
	}
	if !reflect.DeepEqual(doc.Metadata, got.Metadata) {
		t.Fatalf("metadata mismatch")
	}
	if len(got.Media.Items) != 0 || got.Media.BundleVersion != VersionV1 {
		t.Fatalf("expected empty media bundle v1")
	}
}

func TestDecompressPayload_UncompressedLenLimitExceeded(t *testing.T) {
	payload := make([]byte, 8)
	binary.LittleEndian.PutUint64(payload[:8], 10)
	_, err := decompressPayload(CompZSTD, uint16(CompZSTD)|sectionFlagHasUncompressedLen, payload, 1)
	if err == nil || !errors.Is(err, ErrLimitExceeded) {
		t.Fatalf("expected ErrLimitExceeded, got %v", err)
	}
}

func TestCompressPayload_UnknownCompression(t *testing.T) {
	_, _, err := compressPayload(Compression(99), []byte("x"))
	if err == nil {
		t.Fatal("expected error")
	}
}

func TestValidateDocumentFailures(t *testing.T) {
	l := defaultLimits()
	l.MaxMarkdownFiles = 0
	doc := sampleDoc()
	if err := validateDocument(doc, l, false); err == nil {
		t.Fatal("expected error")
	}

	doc = sampleDoc()
	doc.Markdown.Files[0].Path = "/abs.md"
	if err := validateDocument(doc, defaultLimits(), false); err == nil {
		t.Fatal("expected error")
	}

	doc = sampleDoc()
	doc.Media.Items[0].ID = ""
	if err := validateDocument(doc, defaultLimits(), false); err == nil {
		t.Fatal("expected error")
	}

	doc = sampleDoc()
	doc.Media.Items[0].SHA256 = [32]byte{1}
	if err := validateDocument(doc, defaultLimits(), true); err == nil {
		t.Fatal("expected error")
	}

	doc = sampleDoc()
	doc.Markdown.Files = append(doc.Markdown.Files, doc.Markdown.Files[0])
	if err := validateDocument(doc, defaultLimits(), false); err == nil {
		t.Fatal("expected error")
	}

	doc = sampleDoc()
	doc.Media.Items = append(doc.Media.Items, doc.Media.Items[0])
	if err := validateDocument(doc, defaultLimits(), false); err == nil {
		t.Fatal("expected error")
	}

	doc = sampleDoc()
	doc.Markdown.Files[0].Content = []byte{0xff}
	if err := validateDocument(doc, defaultLimits(), false); err == nil {
		t.Fatal("expected error")
	}

	doc = sampleDoc()
	lim := defaultLimits()
	lim.MaxSingleMediaSize = 1
	if err := validateDocument(doc, lim, false); err == nil {
		t.Fatal("expected error")
	}
}

func compressionName(c Compression) string {
	switch c {
	case CompNone:
		return "none"
	case CompZIP:
		return "zip"
	case CompZSTD:
		return "zstd"
	case CompLZ4:
		return "lz4"
	case CompBR:
		return "br"
	default:
		return "unknown"
	}
}
