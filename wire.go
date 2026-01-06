package mdocx

import (
	"encoding/binary"
	"fmt"
	"io"
)

// fixedHeaderV1 represents the 32-byte fixed header at the start of an MDOCX file.
// All integer fields are little-endian encoded.
type fixedHeaderV1 struct {
	Magic          [8]byte // File signature: "MDOCX\r\n" + 0x1A
	Version        uint16  // Format version (must be 1)
	HeaderFlags    uint16  // Flags (bit 0 = METADATA_JSON)
	FixedHdrSize   uint32  // Must be 32
	MetadataLength uint32  // Length of metadata block in bytes
	Reserved0      uint32  // Must be 0 for v1
	Reserved1      uint64  // Must be 0 for v1
}

// sectionHeaderV1 represents the 16-byte header that precedes each section payload.
// All integer fields are little-endian encoded.
type sectionHeaderV1 struct {
	SectionType  uint16 // 1 = Markdown, 2 = Media
	SectionFlags uint16 // Compression algorithm and flags
	PayloadLen   uint64 // Payload length in bytes
	Reserved     uint32 // Must be 0 for v1
}

// readFixedHeader reads and parses the 32-byte fixed header from r.
func readFixedHeader(r io.Reader) (fixedHeaderV1, error) {
	var buf [fixedHeaderSizeV1]byte
	if _, err := io.ReadFull(r, buf[:]); err != nil {
		return fixedHeaderV1{}, err
	}
	var h fixedHeaderV1
	copy(h.Magic[:], buf[0:8])
	h.Version = binary.LittleEndian.Uint16(buf[8:10])
	h.HeaderFlags = binary.LittleEndian.Uint16(buf[10:12])
	h.FixedHdrSize = binary.LittleEndian.Uint32(buf[12:16])
	h.MetadataLength = binary.LittleEndian.Uint32(buf[16:20])
	h.Reserved0 = binary.LittleEndian.Uint32(buf[20:24])
	h.Reserved1 = binary.LittleEndian.Uint64(buf[24:32])
	return h, nil
}

// writeFixedHeader serializes and writes the 32-byte fixed header to w.
func writeFixedHeader(w io.Writer, h fixedHeaderV1) error {
	var buf [fixedHeaderSizeV1]byte
	copy(buf[0:8], h.Magic[:])
	binary.LittleEndian.PutUint16(buf[8:10], h.Version)
	binary.LittleEndian.PutUint16(buf[10:12], h.HeaderFlags)
	binary.LittleEndian.PutUint32(buf[12:16], h.FixedHdrSize)
	binary.LittleEndian.PutUint32(buf[16:20], h.MetadataLength)
	binary.LittleEndian.PutUint32(buf[20:24], h.Reserved0)
	binary.LittleEndian.PutUint64(buf[24:32], h.Reserved1)
	_, err := w.Write(buf[:])
	return err
}

// readSectionHeader reads and parses a 16-byte section header from r.
func readSectionHeader(r io.Reader) (sectionHeaderV1, error) {
	var buf [16]byte
	if _, err := io.ReadFull(r, buf[:]); err != nil {
		return sectionHeaderV1{}, err
	}
	var sh sectionHeaderV1
	sh.SectionType = binary.LittleEndian.Uint16(buf[0:2])
	sh.SectionFlags = binary.LittleEndian.Uint16(buf[2:4])
	sh.PayloadLen = binary.LittleEndian.Uint64(buf[4:12])
	sh.Reserved = binary.LittleEndian.Uint32(buf[12:16])
	return sh, nil
}

// writeSectionHeader serializes and writes a 16-byte section header to w.
func writeSectionHeader(w io.Writer, sh sectionHeaderV1) error {
	var buf [16]byte
	binary.LittleEndian.PutUint16(buf[0:2], sh.SectionType)
	binary.LittleEndian.PutUint16(buf[2:4], sh.SectionFlags)
	binary.LittleEndian.PutUint64(buf[4:12], sh.PayloadLen)
	binary.LittleEndian.PutUint32(buf[12:16], sh.Reserved)
	_, err := w.Write(buf[:])
	return err
}

// compression extracts the compression algorithm from the section flags.
func (sh sectionHeaderV1) compression() Compression {
	return Compression(sh.SectionFlags & sectionFlagCompressionMask)
}

// hasUncompressedLen returns whether the HAS_UNCOMPRESSED_LEN flag is set.
func (sh sectionHeaderV1) hasUncompressedLen() bool {
	return (sh.SectionFlags & sectionFlagHasUncompressedLen) != 0
}

// validateSectionHeader validates that a section header is well-formed and has the expected type.
// It checks the section type, reserved fields, and compression flag consistency.
func validateSectionHeader(sh sectionHeaderV1, expected SectionType) error {
	if sh.Reserved != 0 {
		return fmt.Errorf("%w: reserved must be 0", ErrInvalidSection)
	}
	if SectionType(sh.SectionType) != expected {
		return fmt.Errorf("%w: expected section type %d got %d", ErrInvalidSection, expected, sh.SectionType)
	}
	comp := sh.compression()
	switch comp {
	case CompNone, CompZIP, CompZSTD, CompLZ4, CompBR:
	default:
		return fmt.Errorf("%w: unknown compression %d", ErrInvalidSection, comp)
	}
	if comp == CompNone {
		if sh.hasUncompressedLen() {
			return fmt.Errorf("%w: COMP_NONE must not set HAS_UNCOMPRESSED_LEN", ErrInvalidSection)
		}
	} else {
		if !sh.hasUncompressedLen() {
			return fmt.Errorf("%w: compressed payload must set HAS_UNCOMPRESSED_LEN", ErrInvalidSection)
		}
	}
	return nil
}
