package mdocx

import (
	"archive/zip"
	"bytes"
	"encoding/binary"
	"errors"
	"io"
	"testing"

	"github.com/andybalholm/brotli"
	"github.com/klauspost/compress/zstd"
	"github.com/pierrec/lz4/v4"
)

type errWriter struct{}

func (errWriter) Write(p []byte) (int, error) { return 0, io.ErrClosedPipe }

type errAfterWriter struct {
	remaining int
}

func (w *errAfterWriter) Write(p []byte) (int, error) {
	if w.remaining <= 0 {
		return 0, io.ErrClosedPipe
	}
	if len(p) > w.remaining {
		return 0, io.ErrClosedPipe
	}
	w.remaining -= len(p)
	return len(p), nil
}

func TestCompressHelpers_ErrorPaths(t *testing.T) {
	// zip Create error via injection
	origCreate := zipCreate
	zipCreate = func(_ *zip.Writer, _ string) (io.Writer, error) { return nil, io.ErrClosedPipe }
	if err := zipCompressNamed(io.Discard, "payload.gob", []byte("x")); err == nil {
		zipCreate = origCreate
		t.Fatal("expected error")
	}
	zipCreate = origCreate

	// zip entry.Write error branch: make Create succeed but return a writer that errors on Write.
	origCreate = zipCreate
	zipCreate = func(_ *zip.Writer, _ string) (io.Writer, error) { return errWriter{}, nil }
	if err := zipCompressNamed(io.Discard, "payload.gob", []byte("x")); err == nil {
		zipCreate = origCreate
		t.Fatal("expected error")
	}
	zipCreate = origCreate

	// zip Close error via injection
	origClose := zipClose
	zipClose = func(_ *zip.Writer) error { return io.ErrClosedPipe }
	if err := zipCompressNamed(io.Discard, "payload.gob", []byte("x")); err == nil {
		zipClose = origClose
		t.Fatal("expected error")
	}
	zipClose = origClose

	// zip write error
	if err := zipCompressNamed(errWriter{}, "payload.gob", []byte("x")); err == nil {
		t.Fatal("expected error")
	}
	// lz4 write error
	if err := lz4CompressTo(errWriter{}, []byte("x")); err == nil {
		t.Fatal("expected error")
	}
	// lz4 Close error via injection
	origLZ4Close := lz4Close
	lz4Close = func(_ *lz4.Writer) error { return io.ErrClosedPipe }
	if err := lz4CompressTo(io.Discard, []byte("x")); err == nil {
		lz4Close = origLZ4Close
		t.Fatal("expected error")
	}
	lz4Close = origLZ4Close

	// brotli write error
	origBrotliWrite := brotliWrite
	brotliWrite = func(_ *brotli.Writer, _ []byte) (int, error) { return 0, io.ErrClosedPipe }
	if err := brotliCompressTo(io.Discard, []byte("x")); err == nil {
		brotliWrite = origBrotliWrite
		t.Fatal("expected error")
	}
	brotliWrite = origBrotliWrite
	// brotli Close error via injection
	origBrotliClose := brotliClose
	brotliClose = func(_ *brotli.Writer) error { return io.ErrClosedPipe }
	if err := brotliCompressTo(io.Discard, []byte("x")); err == nil {
		brotliClose = origBrotliClose
		t.Fatal("expected error")
	}
	brotliClose = origBrotliClose
}

func TestBrotliDecompress_ReadAllError(t *testing.T) {
	orig := readAll
	readAll = func(io.Reader) ([]byte, error) { return nil, io.ErrClosedPipe }
	defer func() { readAll = orig }()
	if _, err := brotliDecompress([]byte("anything"), 10); err == nil {
		t.Fatal("expected error")
	}
}

func TestZstdConstructorInjection(t *testing.T) {
	origW := newZstdWriter
	origR := newZstdReader
	defer func() {
		newZstdWriter = origW
		newZstdReader = origR
	}()

	newZstdWriter = func() (*zstd.Encoder, error) { return nil, io.ErrClosedPipe }
	if _, err := zstdCompress([]byte("x")); err == nil {
		t.Fatal("expected error")
	}

	newZstdWriter = origW
	newZstdReader = func() (*zstd.Decoder, error) { return nil, io.ErrClosedPipe }
	if _, err := zstdDecompress([]byte("x"), 10); err == nil {
		t.Fatal("expected error")
	}
}

func TestZIPDecompress_InjectionErrorPaths(t *testing.T) {
	z, err := zipCompress([]byte("abc"))
	if err != nil {
		t.Fatal(err)
	}
	origOpen := zipOpen
	zipOpen = func(_ *zip.File) (io.ReadCloser, error) { return nil, io.ErrClosedPipe }
	if _, err := zipDecompress(z, 3); err == nil {
		zipOpen = origOpen
		t.Fatal("expected error")
	}
	zipOpen = origOpen

	origReadAll := readAll
	readAll = func(io.Reader) ([]byte, error) { return nil, io.ErrClosedPipe }
	if _, err := zipDecompress(z, 3); err == nil {
		readAll = origReadAll
		t.Fatal("expected error")
	}
	readAll = origReadAll
}

func TestCompressPayload_UnderlyingError(t *testing.T) {
	origW := newZstdWriter
	defer func() { newZstdWriter = origW }()
	newZstdWriter = func() (*zstd.Encoder, error) { return nil, io.ErrClosedPipe }
	_, _, err := compressPayload(CompZSTD, []byte("x"))
	if err == nil {
		t.Fatal("expected error")
	}
}

func TestDecompressPayload_UnderlyingError(t *testing.T) {
	// invalid ZIP bytes through decompressPayload
	payload := make([]byte, 8)
	binary.LittleEndian.PutUint64(payload[:8], 3)
	payload = append(payload, []byte("notzip")...)
	_, err := decompressPayload(CompZIP, uint16(CompZIP)|sectionFlagHasUncompressedLen, payload, 100)
	if err == nil {
		t.Fatal("expected error")
	}
}

func TestDecode_TruncatedInputs(t *testing.T) {
	// Truncated fixed header
	if _, err := Decode(bytes.NewReader([]byte("short"))); err == nil {
		t.Fatal("expected error")
	}

	// Truncated metadata
	var buf bytes.Buffer
	h := fixedHeaderV1{Magic: Magic, Version: VersionV1, HeaderFlags: HeaderFlagMetadataJSON, FixedHdrSize: fixedHeaderSizeV1, MetadataLength: 10}
	if err := writeFixedHeader(&buf, h); err != nil {
		t.Fatal(err)
	}
	buf.WriteString("{}") // too short
	if _, err := Decode(bytes.NewReader(buf.Bytes())); err == nil {
		t.Fatal("expected error")
	}

	// Truncated section header
	buf.Reset()
	h = fixedHeaderV1{Magic: Magic, Version: VersionV1, FixedHdrSize: fixedHeaderSizeV1}
	_ = writeFixedHeader(&buf, h)
	buf.Write([]byte{0x01, 0x02})
	if _, err := Decode(bytes.NewReader(buf.Bytes())); err == nil {
		t.Fatal("expected error")
	}
}

func TestDecode_TruncatedPayloads(t *testing.T) {
	// Markdown payload advertised but missing.
	var buf bytes.Buffer
	h := fixedHeaderV1{Magic: Magic, Version: VersionV1, FixedHdrSize: fixedHeaderSizeV1}
	_ = writeFixedHeader(&buf, h)
	mdHeader := sectionHeaderV1{SectionType: uint16(SectionMarkdown), SectionFlags: uint16(CompNone), PayloadLen: 5, Reserved: 0}
	_ = writeSectionHeader(&buf, mdHeader)
	if _, err := Decode(bytes.NewReader(buf.Bytes())); err == nil {
		t.Fatal("expected error")
	}
}

func TestDecode_InvalidGob(t *testing.T) {
	var buf bytes.Buffer
	h := fixedHeaderV1{Magic: Magic, Version: VersionV1, FixedHdrSize: fixedHeaderSizeV1}
	_ = writeFixedHeader(&buf, h)

	mdHeader := sectionHeaderV1{SectionType: uint16(SectionMarkdown), SectionFlags: uint16(CompNone), PayloadLen: 1, Reserved: 0}
	_ = writeSectionHeader(&buf, mdHeader)
	buf.WriteByte(0xFF) // invalid gob

	// Media header must be present for structural correctness, but Decode should fail on markdown gob first.
	mediaHeader := sectionHeaderV1{SectionType: uint16(SectionMedia), SectionFlags: uint16(CompNone), PayloadLen: 0, Reserved: 0}
	_ = writeSectionHeader(&buf, mediaHeader)

	if _, err := Decode(bytes.NewReader(buf.Bytes())); err == nil {
		t.Fatal("expected error")
	}
}

func TestEncode_ErrorPositions(t *testing.T) {
	doc := sampleDoc()
	// Fail on first write (fixed header)
	if err := Encode(&errAfterWriter{remaining: 0}, doc, WithMarkdownCompression(CompNone), WithMediaCompression(CompNone)); err == nil {
		t.Fatal("expected error")
	}
	// Fail writing metadata
	if err := Encode(&errAfterWriter{remaining: 32}, doc, WithMarkdownCompression(CompNone), WithMediaCompression(CompNone)); err == nil {
		t.Fatal("expected error")
	}
	// Fail writing markdown section header
	noMeta := sampleDoc()
	noMeta.Metadata = nil
	if err := Encode(&errAfterWriter{remaining: 32}, noMeta, WithMarkdownCompression(CompNone), WithMediaCompression(CompNone)); err == nil {
		t.Fatal("expected error")
	}
	// Fail writing markdown payload
	var tmp bytes.Buffer
	_ = Encode(&tmp, noMeta, WithMarkdownCompression(CompNone), WithMediaCompression(CompNone))
	b := tmp.Bytes()
	metaLen := int(binary.LittleEndian.Uint32(b[16:20]))
	mdOff := 32 + metaLen
	mdPayloadLen := int(binary.LittleEndian.Uint64(b[mdOff+4 : mdOff+12]))
	if err := Encode(&errAfterWriter{remaining: 32 + 16 + mdPayloadLen - 1}, noMeta, WithMarkdownCompression(CompNone), WithMediaCompression(CompNone)); err == nil {
		t.Fatal("expected error")
	}
	// Unknown compression through Encode
	if err := Encode(io.Discard, noMeta, WithMarkdownCompression(Compression(99))); err == nil {
		t.Fatal("expected error")
	}
	if err := Encode(io.Discard, noMeta, WithMarkdownCompression(CompNone), WithMediaCompression(Compression(99))); err == nil {
		t.Fatal("expected error")
	}
}

func TestEncode_GobEncodeInjectedErrors(t *testing.T) {
	noMeta := sampleDoc()
	noMeta.Metadata = nil
	noMeta.Media.Items = nil

	origMD := gobEncodeMarkdown
	origMedia := gobEncodeMedia
	defer func() {
		gobEncodeMarkdown = origMD
		gobEncodeMedia = origMedia
	}()

	gobEncodeMarkdown = func(MarkdownBundle) ([]byte, error) { return nil, io.ErrClosedPipe }
	if err := Encode(io.Discard, noMeta, WithMarkdownCompression(CompNone), WithMediaCompression(CompNone)); err == nil {
		t.Fatal("expected error")
	}

	gobEncodeMarkdown = origMD
	gobEncodeMedia = func(MediaBundle) ([]byte, error) { return nil, io.ErrClosedPipe }
	if err := Encode(io.Discard, noMeta, WithMarkdownCompression(CompNone), WithMediaCompression(CompNone)); err == nil {
		t.Fatal("expected error")
	}
}

func TestEncode_MetadataMarshalError(t *testing.T) {
	d := sampleDoc()
	d.Metadata = map[string]any{"bad": func() {}}
	if err := Encode(io.Discard, d, WithMarkdownCompression(CompNone), WithMediaCompression(CompNone)); err == nil {
		t.Fatal("expected error")
	}
}

func TestEncode_AutoPopulateBranchNotTaken(t *testing.T) {
	d := sampleDoc()
	d.Metadata = nil
	// Set a non-zero hash so the auto-populate inner branch is not taken.
	d.Media.Items[0].SHA256 = [32]byte{1}
	var buf bytes.Buffer
	if err := Encode(&buf, d, WithVerifyHashesOnWrite(false), WithMarkdownCompression(CompNone), WithMediaCompression(CompNone)); err != nil {
		t.Fatal(err)
	}
}

func TestEncode_AutoPopulateSetsSHA256(t *testing.T) {
	d := sampleDoc()
	d.Metadata = nil
	d.Media.Items[0].SHA256 = [32]byte{}
	if err := Encode(io.Discard, d, WithMarkdownCompression(CompNone), WithMediaCompression(CompNone)); err != nil {
		t.Fatal(err)
	}
	if d.Media.Items[0].SHA256 == ([32]byte{}) {
		t.Fatal("expected SHA256 to be populated")
	}
}

func TestEncode_ValidationErrorReturned(t *testing.T) {
	d := sampleDoc()
	d.Metadata = nil
	d.Markdown.BundleVersion = 2
	if err := Encode(io.Discard, d, WithMarkdownCompression(CompNone), WithMediaCompression(CompNone)); err == nil {
		t.Fatal("expected error")
	}
}

func TestEncode_MediaHeaderAndPayloadWriteErrors(t *testing.T) {
	d := sampleDoc()
	d.Metadata = nil
	var tmp bytes.Buffer
	if err := Encode(&tmp, d, WithMarkdownCompression(CompNone), WithMediaCompression(CompNone)); err != nil {
		t.Fatal(err)
	}
	b := tmp.Bytes()
	mdOff := 32
	mdPayloadLen := int(binary.LittleEndian.Uint64(b[mdOff+4 : mdOff+12]))
	// After: fixed header (32) + md header (16) + md payload
	bytesBeforeMediaHeader := 32 + 16 + mdPayloadLen
	if err := Encode(&errAfterWriter{remaining: bytesBeforeMediaHeader}, d, WithMarkdownCompression(CompNone), WithMediaCompression(CompNone)); err == nil {
		t.Fatal("expected error")
	}
	// Fail writing media payload (after media header)
	if err := Encode(&errAfterWriter{remaining: bytesBeforeMediaHeader + 16}, d, WithMarkdownCompression(CompNone), WithMediaCompression(CompNone)); err == nil {
		t.Fatal("expected error")
	}
}

func TestGobEncode_Error(t *testing.T) {
	type bad struct{ Ch chan int }
	if _, err := gobEncode(bad{Ch: make(chan int)}); err == nil {
		t.Fatal("expected error")
	}
}

func TestOptions_VerifyAndAutopopulate(t *testing.T) {
	doc := sampleDoc()
	// Disable auto-population and disable write-time verification so an incorrect hash is written.
	doc.Media.Items[0].SHA256 = [32]byte{1}
	var buf bytes.Buffer
	if err := Encode(&buf, doc, WithAutoPopulateSHA256(false), WithVerifyHashesOnWrite(false), WithMarkdownCompression(CompNone), WithMediaCompression(CompNone)); err != nil {
		t.Fatal(err)
	}
	// Default Decode verifies hashes -> should fail.
	if _, err := Decode(bytes.NewReader(buf.Bytes())); err == nil {
		t.Fatal("expected error")
	}
	// Disable hash verification -> should succeed.
	if _, err := Decode(bytes.NewReader(buf.Bytes()), WithVerifyHashes(false)); err != nil {
		t.Fatalf("expected success, got %v", err)
	}
}

func TestValidateDocument_Nil(t *testing.T) {
	if err := validateDocument(nil, defaultLimits(), false); err == nil {
		t.Fatal("expected error")
	}
}

func TestZipDecompress_BadArchive(t *testing.T) {
	_, err := zipDecompress([]byte("not a zip"), 1)
	if err == nil {
		t.Fatal("expected error")
	}
}

func TestDecompressPayload_UnknownCompression(t *testing.T) {
	_, err := decompressPayload(Compression(99), uint16(99)|sectionFlagHasUncompressedLen, make([]byte, 8), 100)
	if err == nil {
		t.Fatal("expected error")
	}
	if !errors.Is(err, ErrInvalidPayload) {
		t.Fatalf("expected ErrInvalidPayload, got %v", err)
	}
}
