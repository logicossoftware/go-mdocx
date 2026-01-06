package mdocx

import (
	"archive/zip"
	"bytes"
	"encoding/binary"
	"fmt"
	"io"

	"github.com/andybalholm/brotli"
	"github.com/klauspost/compress/zstd"
	"github.com/pierrec/lz4/v4"
)

// Function variables for testing injection.
var (
	newZstdWriter = func() (*zstd.Encoder, error) { return zstd.NewWriter(nil) }
	newZstdReader = func() (*zstd.Decoder, error) { return zstd.NewReader(nil) }
	zipCreate     = func(zw *zip.Writer, name string) (io.Writer, error) { return zw.Create(name) }
	zipClose      = func(zw *zip.Writer) error { return zw.Close() }
	zipOpen       = func(zf *zip.File) (io.ReadCloser, error) { return zf.Open() }
	readAll       = io.ReadAll
	lz4Close      = func(w *lz4.Writer) error { return w.Close() }
	brotliClose   = func(w *brotli.Writer) error { return w.Close() }
	brotliWrite   = func(w *brotli.Writer, p []byte) (int, error) { return w.Write(p) }
)

// compressPayload compresses gobBytes using the specified compression algorithm.
// It returns the section flags (with compression bits set) and the payload bytes.
// For compressed payloads, the payload includes an 8-byte uncompressed length prefix.
func compressPayload(comp Compression, gobBytes []byte) (sectionFlags uint16, payload []byte, err error) {
	if comp == CompNone {
		return uint16(CompNone), gobBytes, nil
	}
	var compressed []byte
	switch comp {
	case CompZIP:
		compressed, err = zipCompress(gobBytes)
	case CompZSTD:
		compressed, err = zstdCompress(gobBytes)
	case CompLZ4:
		compressed, err = lz4Compress(gobBytes)
	case CompBR:
		compressed, err = brotliCompress(gobBytes)
	default:
		return 0, nil, fmt.Errorf("%w: unknown compression %d", ErrInvalidPayload, comp)
	}
	if err != nil {
		return 0, nil, err
	}
	var prefix [8]byte
	binary.LittleEndian.PutUint64(prefix[:], uint64(len(gobBytes)))
	payload = append(prefix[:], compressed...)
	sectionFlags = uint16(comp) | sectionFlagHasUncompressedLen
	return sectionFlags, payload, nil
}

// decompressPayload decompresses payload bytes based on the compression algorithm.
// It enforces maxUncompressed to prevent decompression bombs.
// For CompNone, the payload is returned as-is.
// For all other algorithms, the payload must start with an 8-byte uncompressed length prefix.
func decompressPayload(comp Compression, sectionFlags uint16, payload []byte, maxUncompressed uint64) ([]byte, error) {
	hasLen := (sectionFlags & sectionFlagHasUncompressedLen) != 0
	if comp == CompNone {
		if hasLen {
			return nil, fmt.Errorf("%w: COMP_NONE with HAS_UNCOMPRESSED_LEN", ErrInvalidPayload)
		}
		return payload, nil
	}
	if !hasLen {
		return nil, fmt.Errorf("%w: missing HAS_UNCOMPRESSED_LEN", ErrInvalidPayload)
	}
	if len(payload) < 8 {
		return nil, fmt.Errorf("%w: payload too short for uncompressed length", ErrInvalidPayload)
	}
	uncompressedLen := binary.LittleEndian.Uint64(payload[:8])
	if uncompressedLen > maxUncompressed {
		return nil, fmt.Errorf("%w: uncompressed length %d exceeds limit", ErrLimitExceeded, uncompressedLen)
	}
	compressedBytes := payload[8:]

	var out []byte
	var err error
	switch comp {
	case CompZIP:
		out, err = zipDecompress(compressedBytes, uncompressedLen)
	case CompZSTD:
		out, err = zstdDecompress(compressedBytes, uncompressedLen)
	case CompLZ4:
		out, err = lz4Decompress(compressedBytes, uncompressedLen)
	case CompBR:
		out, err = brotliDecompress(compressedBytes, uncompressedLen)
	default:
		return nil, fmt.Errorf("%w: unknown compression %d", ErrInvalidPayload, comp)
	}
	if err != nil {
		return nil, err
	}
	if uint64(len(out)) != uncompressedLen {
		return nil, fmt.Errorf("%w: decompressed length %d != expected %d", ErrInvalidPayload, len(out), uncompressedLen)
	}
	return out, nil
}

// zipCompress creates a ZIP archive containing in as "payload.gob".
func zipCompress(in []byte) ([]byte, error) {
	var buf bytes.Buffer
	if err := zipCompressNamed(&buf, "payload.gob", in); err != nil {
		return nil, err
	}
	return buf.Bytes(), nil
}

// zipCompressNamed creates a ZIP archive with a single entry.
func zipCompressNamed(w io.Writer, name string, in []byte) error {
	zw := zip.NewWriter(w)
	entry, err := zipCreate(zw, name)
	if err != nil {
		_ = zipClose(zw)
		return err
	}
	if _, err := entry.Write(in); err != nil {
		_ = zipClose(zw)
		return err
	}
	return zipClose(zw)
}

// zipDecompress extracts the "payload.gob" entry from a ZIP archive.
// It validates that the archive contains exactly one entry named "payload.gob"
// and that the uncompressed size matches expected.
func zipDecompress(zipBytes []byte, expected uint64) ([]byte, error) {
	zr, err := zip.NewReader(bytes.NewReader(zipBytes), int64(len(zipBytes)))
	if err != nil {
		return nil, err
	}
	if len(zr.File) != 1 {
		return nil, fmt.Errorf("%w: zip must contain exactly one entry", ErrInvalidPayload)
	}
	zf := zr.File[0]
	if zf.Name != "payload.gob" {
		return nil, fmt.Errorf("%w: zip entry name must be payload.gob", ErrInvalidPayload)
	}
	if zf.FileInfo().IsDir() {
		return nil, fmt.Errorf("%w: zip entry must be a file", ErrInvalidPayload)
	}
	// Reject unknown sizes if possible.
	if zf.UncompressedSize64 != expected {
		return nil, fmt.Errorf("%w: zip uncompressed size %d != expected %d", ErrInvalidPayload, zf.UncompressedSize64, expected)
	}
	rc, err := zipOpen(zf)
	if err != nil {
		return nil, err
	}
	defer rc.Close()
	b, err := readAll(io.LimitReader(rc, int64(expected)))
	if err != nil {
		return nil, err
	}
	return b, nil
}

// zstdCompress compresses in using the Zstandard algorithm.
func zstdCompress(in []byte) ([]byte, error) {
	enc, err := newZstdWriter()
	if err != nil {
		return nil, err
	}
	defer enc.Close()
	return enc.EncodeAll(in, nil), nil
}

// zstdDecompress decompresses Zstandard-compressed data.
// It rejects output that exceeds expected bytes.
func zstdDecompress(in []byte, expected uint64) ([]byte, error) {
	dec, err := newZstdReader()
	if err != nil {
		return nil, err
	}
	defer dec.Close()
	out, err := dec.DecodeAll(in, nil)
	if err != nil {
		return nil, err
	}
	if uint64(len(out)) > expected {
		return nil, fmt.Errorf("%w: zstd expanded beyond expected size", ErrInvalidPayload)
	}
	return out, nil
}

// lz4Compress compresses in using the LZ4 algorithm.
func lz4Compress(in []byte) ([]byte, error) {
	var buf bytes.Buffer
	if err := lz4CompressTo(&buf, in); err != nil {
		return nil, err
	}
	return buf.Bytes(), nil
}

// lz4CompressTo writes LZ4-compressed data to w.
func lz4CompressTo(w io.Writer, in []byte) error {
	zw := lz4.NewWriter(w)
	if _, err := zw.Write(in); err != nil {
		_ = lz4Close(zw)
		return err
	}
	return lz4Close(zw)
}

// lz4Decompress decompresses LZ4-compressed data.
// It uses a LimitReader to prevent decompression beyond expected bytes.
func lz4Decompress(in []byte, expected uint64) ([]byte, error) {
	r := lz4.NewReader(bytes.NewReader(in))
	b, err := io.ReadAll(io.LimitReader(r, int64(expected)+1))
	if err != nil {
		return nil, err
	}
	if uint64(len(b)) > expected {
		return nil, fmt.Errorf("%w: lz4 expanded beyond expected size", ErrInvalidPayload)
	}
	return b, nil
}

// brotliCompress compresses in using the Brotli algorithm.
func brotliCompress(in []byte) ([]byte, error) {
	var buf bytes.Buffer
	if err := brotliCompressTo(&buf, in); err != nil {
		return nil, err
	}
	return buf.Bytes(), nil
}

// brotliCompressTo writes Brotli-compressed data to w.
func brotliCompressTo(w io.Writer, in []byte) error {
	bw := brotli.NewWriter(w)
	if _, err := brotliWrite(bw, in); err != nil {
		_ = brotliClose(bw)
		return err
	}
	return brotliClose(bw)
}

// brotliDecompress decompresses Brotli-compressed data.
// It uses a LimitReader to prevent decompression beyond expected bytes.
func brotliDecompress(in []byte, expected uint64) ([]byte, error) {
	r := brotli.NewReader(bytes.NewReader(in))
	b, err := readAll(io.LimitReader(r, int64(expected)+1))
	if err != nil {
		return nil, err
	}
	if uint64(len(b)) > expected {
		return nil, fmt.Errorf("%w: brotli expanded beyond expected size", ErrInvalidPayload)
	}
	return b, nil
}
