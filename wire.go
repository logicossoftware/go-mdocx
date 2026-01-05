package mdocx

import (
	"encoding/binary"
	"fmt"
	"io"
)

type fixedHeaderV1 struct {
	Magic          [8]byte
	Version        uint16
	HeaderFlags    uint16
	FixedHdrSize   uint32
	MetadataLength uint32
	Reserved0      uint32
	Reserved1      uint64
}

type sectionHeaderV1 struct {
	SectionType  uint16
	SectionFlags uint16
	PayloadLen   uint64
	Reserved     uint32
}

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

func writeSectionHeader(w io.Writer, sh sectionHeaderV1) error {
	var buf [16]byte
	binary.LittleEndian.PutUint16(buf[0:2], sh.SectionType)
	binary.LittleEndian.PutUint16(buf[2:4], sh.SectionFlags)
	binary.LittleEndian.PutUint64(buf[4:12], sh.PayloadLen)
	binary.LittleEndian.PutUint32(buf[12:16], sh.Reserved)
	_, err := w.Write(buf[:])
	return err
}

func (sh sectionHeaderV1) compression() Compression {
	return Compression(sh.SectionFlags & sectionFlagCompressionMask)
}

func (sh sectionHeaderV1) hasUncompressedLen() bool {
	return (sh.SectionFlags & sectionFlagHasUncompressedLen) != 0
}

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
