package mdocx

import (
	"archive/zip"
	"bytes"
	"encoding/binary"
	"io"
	"io/fs"
	"testing"

	"github.com/andybalholm/brotli"
	"github.com/pierrec/lz4/v4"
)

func TestZIPDecompressErrors(t *testing.T) {
	// Multi-entry
	{
		var buf bytes.Buffer
		zw := zip.NewWriter(&buf)
		_, _ = zw.Create("payload.gob")
		_, _ = zw.Create("extra")
		_ = zw.Close()
		_, err := zipDecompress(buf.Bytes(), 0)
		if err == nil {
			t.Fatal("expected error")
		}
	}
	// Wrong name
	{
		var buf bytes.Buffer
		zw := zip.NewWriter(&buf)
		w, _ := zw.Create("nope")
		_, _ = w.Write([]byte("abc"))
		_ = zw.Close()
		_, err := zipDecompress(buf.Bytes(), 3)
		if err == nil {
			t.Fatal("expected error")
		}
	}
	// Size mismatch
	{
		var buf bytes.Buffer
		zw := zip.NewWriter(&buf)
		w, _ := zw.Create("payload.gob")
		_, _ = w.Write([]byte("abcd"))
		_ = zw.Close()
		_, err := zipDecompress(buf.Bytes(), 3)
		if err == nil {
			t.Fatal("expected error")
		}
	}
	// Entry is a directory
	{
		var buf bytes.Buffer
		zw := zip.NewWriter(&buf)
		h := &zip.FileHeader{Name: "payload.gob"}
		h.SetMode(fs.ModeDir | 0o755)
		_, _ = zw.CreateHeader(h)
		_ = zw.Close()
		_, err := zipDecompress(buf.Bytes(), 0)
		if err == nil {
			t.Fatal("expected error")
		}
	}
}

func TestDecompressionExpansionGuards(t *testing.T) {
	in := []byte("hello world")

	zst, err := zstdCompress(in)
	if err != nil {
		t.Fatal(err)
	}
	if _, err := zstdDecompress(zst, 1); err == nil {
		t.Fatal("expected error")
	}

	lz, err := lz4Compress(in)
	if err != nil {
		t.Fatal(err)
	}
	if _, err := lz4Decompress(lz, 1); err == nil {
		t.Fatal("expected error")
	}

	br, err := brotliCompress(in)
	if err != nil {
		t.Fatal(err)
	}
	if _, err := brotliDecompress(br, 1); err == nil {
		t.Fatal("expected error")
	}
}

func TestCompressionWrappers_ReturnErrors(t *testing.T) {
	// zipCompress wrapper error
	origCreate := zipCreate
	zipCreate = func(_ *zip.Writer, _ string) (io.Writer, error) { return nil, io.ErrClosedPipe }
	if _, err := zipCompress([]byte("x")); err == nil {
		zipCreate = origCreate
		t.Fatal("expected error")
	}
	zipCreate = origCreate

	// lz4Compress wrapper error
	origLZ4Close := lz4Close
	lz4Close = func(_ *lz4.Writer) error { return io.ErrClosedPipe }
	if _, err := lz4Compress([]byte("x")); err == nil {
		lz4Close = origLZ4Close
		t.Fatal("expected error")
	}
	lz4Close = origLZ4Close

	// brotliCompress wrapper error
	origBrotliClose := brotliClose
	brotliClose = func(_ *brotli.Writer) error { return io.ErrClosedPipe }
	if _, err := brotliCompress([]byte("x")); err == nil {
		brotliClose = origBrotliClose
		t.Fatal("expected error")
	}
	brotliClose = origBrotliClose
}

func TestDecompressionCorruptStreams(t *testing.T) {
	// zstd: corrupt stream should error
	if _, err := zstdDecompress([]byte("notzstd"), 100); err == nil {
		t.Fatal("expected error")
	}
	// lz4: corrupt stream should error
	if _, err := lz4Decompress([]byte("notlz4"), 100); err == nil {
		t.Fatal("expected error")
	}
	// brotli: corrupt stream should error
	if _, err := brotliDecompress([]byte("notbr"), 100); err == nil {
		t.Fatal("expected error")
	}
}

func TestDecompressPayloadLengthMismatch(t *testing.T) {
	in := []byte("abc")
	compressed, err := zstdCompress(in)
	if err != nil {
		t.Fatal(err)
	}
	payload := make([]byte, 8+len(compressed))
	binary.LittleEndian.PutUint64(payload[:8], 10)
	copy(payload[8:], compressed)
	_, err = decompressPayload(CompZSTD, uint16(CompZSTD)|sectionFlagHasUncompressedLen, payload, 100)
	if err == nil {
		t.Fatal("expected error")
	}
}

func TestDecompressPayloadBadEnvelope(t *testing.T) {
	if _, err := decompressPayload(CompNone, sectionFlagHasUncompressedLen, []byte("x"), 10); err == nil {
		t.Fatal("expected error")
	}
	if _, err := decompressPayload(CompZSTD, uint16(CompZSTD), []byte("x"), 10); err == nil {
		t.Fatal("expected error")
	}
	if _, err := decompressPayload(CompZSTD, uint16(CompZSTD)|sectionFlagHasUncompressedLen, []byte{1, 2, 3}, 10); err == nil {
		t.Fatal("expected error")
	}
}
