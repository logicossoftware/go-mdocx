package mdocx

import "testing"

func TestSectionHeaderMethods(t *testing.T) {
	sh := sectionHeaderV1{SectionFlags: uint16(CompZSTD) | sectionFlagHasUncompressedLen}
	if sh.compression() != CompZSTD {
		t.Fatal("expected CompZSTD")
	}
	if !sh.hasUncompressedLen() {
		t.Fatal("expected has uncompressed len")
	}
}
