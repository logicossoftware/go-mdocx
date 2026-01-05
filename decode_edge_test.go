package mdocx

import (
	"bytes"
	"encoding/binary"
	"testing"
)

func TestDecode_MetadataNullRejected(t *testing.T) {
	var buf bytes.Buffer
	h := fixedHeaderV1{Magic: Magic, Version: VersionV1, HeaderFlags: HeaderFlagMetadataJSON, FixedHdrSize: fixedHeaderSizeV1, MetadataLength: 4}
	_ = writeFixedHeader(&buf, h)
	buf.WriteString("null")
	_, err := Decode(bytes.NewReader(buf.Bytes()))
	if err == nil {
		t.Fatal("expected error")
	}
}

func TestDecode_TruncatedBeforeMediaHeader(t *testing.T) {
	doc := sampleDoc()
	doc.Metadata = nil
	var buf bytes.Buffer
	if err := Encode(&buf, doc, WithMarkdownCompression(CompNone), WithMediaCompression(CompNone)); err != nil {
		t.Fatal(err)
	}
	b := buf.Bytes()
	mdOff := 32
	mdPayloadLen := int(binary.LittleEndian.Uint64(b[mdOff+4 : mdOff+12]))
	// Cut right after markdown header+payload, removing the media section header.
	cut := mdOff + 16 + mdPayloadLen
	_, err := Decode(bytes.NewReader(b[:cut]))
	if err == nil {
		t.Fatal("expected error")
	}
}

func TestDecode_MediaHeaderInvalid(t *testing.T) {
	doc := sampleDoc()
	doc.Metadata = nil
	var buf bytes.Buffer
	if err := Encode(&buf, doc, WithMarkdownCompression(CompNone), WithMediaCompression(CompNone)); err != nil {
		t.Fatal(err)
	}
	b := buf.Bytes()
	mdOff := 32
	mdPayloadLen := int(binary.LittleEndian.Uint64(b[mdOff+4 : mdOff+12]))
	mediaOff := mdOff + 16 + mdPayloadLen
	// Make media section type wrong.
	binary.LittleEndian.PutUint16(b[mediaOff:mediaOff+2], uint16(SectionMarkdown))
	_, err := Decode(bytes.NewReader(b))
	if err == nil {
		t.Fatal("expected error")
	}
}

func TestDecode_MediaSectionLenLimitExceeded(t *testing.T) {
	doc := sampleDoc()
	doc.Metadata = nil
	var buf bytes.Buffer
	if err := Encode(&buf, doc, WithMarkdownCompression(CompNone), WithMediaCompression(CompNone)); err != nil {
		t.Fatal(err)
	}
	b := buf.Bytes()
	mdOff := 32
	mdPayloadLen := int(binary.LittleEndian.Uint64(b[mdOff+4 : mdOff+12]))
	mediaOff := mdOff + 16 + mdPayloadLen
	binary.LittleEndian.PutUint64(b[mediaOff+4:mediaOff+12], 9999)
	_, err := Decode(bytes.NewReader(b), WithReadLimits(Limits{MaxMediaSectionLen: 1}))
	if err == nil {
		t.Fatal("expected error")
	}
}

func TestDecode_TruncatedMediaPayload(t *testing.T) {
	doc := sampleDoc()
	doc.Metadata = nil
	var buf bytes.Buffer
	if err := Encode(&buf, doc, WithMarkdownCompression(CompNone), WithMediaCompression(CompNone)); err != nil {
		t.Fatal(err)
	}
	b := buf.Bytes()
	mdOff := 32
	mdPayloadLen := int(binary.LittleEndian.Uint64(b[mdOff+4 : mdOff+12]))
	mediaOff := mdOff + 16 + mdPayloadLen
	// Keep media header but remove payload bytes.
	_, err := Decode(bytes.NewReader(b[:mediaOff+16]))
	if err == nil {
		t.Fatal("expected error")
	}
}

func TestDecode_MediaDecompressError(t *testing.T) {
	doc := sampleDoc()
	doc.Metadata = nil
	var buf bytes.Buffer
	if err := Encode(&buf, doc, WithMarkdownCompression(CompNone), WithMediaCompression(CompZIP)); err != nil {
		t.Fatal(err)
	}
	b := buf.Bytes()
	mdOff := 32
	mdPayloadLen := int(binary.LittleEndian.Uint64(b[mdOff+4 : mdOff+12]))
	mediaOff := mdOff + 16 + mdPayloadLen
	mediaPayloadLen := int(binary.LittleEndian.Uint64(b[mediaOff+4 : mediaOff+12]))
	// Corrupt the media payload bytes (after the 8-byte uncompressed length prefix).
	payloadStart := mediaOff + 16
	if mediaPayloadLen > 12 {
		b[payloadStart+10] ^= 0xFF
	}
	_, err := Decode(bytes.NewReader(b))
	if err == nil {
		t.Fatal("expected error")
	}
}

func TestDecode_MediaGobDecodeError(t *testing.T) {
	// Build file with valid markdown gob + invalid media gob.
	md := MarkdownBundle{BundleVersion: VersionV1, Files: []MarkdownFile{{Path: "docs/a.md", Content: []byte("ok\n")}}}
	mdGob, err := gobEncode(md)
	if err != nil {
		t.Fatal(err)
	}
	var buf bytes.Buffer
	h := fixedHeaderV1{Magic: Magic, Version: VersionV1, FixedHdrSize: fixedHeaderSizeV1}
	_ = writeFixedHeader(&buf, h)
	_ = writeSectionHeader(&buf, sectionHeaderV1{SectionType: uint16(SectionMarkdown), SectionFlags: uint16(CompNone), PayloadLen: uint64(len(mdGob))})
	buf.Write(mdGob)
	_ = writeSectionHeader(&buf, sectionHeaderV1{SectionType: uint16(SectionMedia), SectionFlags: uint16(CompNone), PayloadLen: 1})
	buf.WriteByte(0xFF)
	_, err = Decode(bytes.NewReader(buf.Bytes()))
	if err == nil {
		t.Fatal("expected error")
	}
}

func TestDecode_MarkdownPayloadReadFullError(t *testing.T) {
	var buf bytes.Buffer
	h := fixedHeaderV1{Magic: Magic, Version: VersionV1, FixedHdrSize: fixedHeaderSizeV1}
	_ = writeFixedHeader(&buf, h)
	_ = writeSectionHeader(&buf, sectionHeaderV1{SectionType: uint16(SectionMarkdown), SectionFlags: uint16(CompNone), PayloadLen: 5})
	buf.WriteByte(0x01) // too short
	_, err := Decode(bytes.NewReader(buf.Bytes()))
	if err == nil {
		t.Fatal("expected error")
	}
}

func TestDecode_MarkdownDecompressError(t *testing.T) {
	var buf bytes.Buffer
	h := fixedHeaderV1{Magic: Magic, Version: VersionV1, FixedHdrSize: fixedHeaderSizeV1}
	_ = writeFixedHeader(&buf, h)
	// Compressed markdown section with invalid zstd bytes.
	payload := make([]byte, 8)
	binary.LittleEndian.PutUint64(payload[:8], 3)
	payload = append(payload, []byte("notzstd")...)
	_ = writeSectionHeader(&buf, sectionHeaderV1{SectionType: uint16(SectionMarkdown), SectionFlags: uint16(CompZSTD) | sectionFlagHasUncompressedLen, PayloadLen: uint64(len(payload))})
	buf.Write(payload)
	_, err := Decode(bytes.NewReader(buf.Bytes()))
	if err == nil {
		t.Fatal("expected error")
	}
}
