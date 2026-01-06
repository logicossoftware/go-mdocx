// Package main provides C-compatible exports for the mdocx library.
// Build with: go build -buildmode=c-shared -o mdocx.dll
package main

/*
#include <stdlib.h>
#include <stdint.h>

// Result structure for operations that return data
typedef struct {
    char* data;
    int   data_len;
    char* error;
} MdocxResult;

// MarkdownFile for creating documents
typedef struct {
    char* path;
    char* content;
    int   content_len;
} CMarkdownFile;

// MediaItem for creating documents
typedef struct {
    char* id;
    char* path;
    char* mime_type;
    char* data;
    int   data_len;
} CMediaItem;
*/
import "C"

import (
	"bytes"
	"encoding/json"
	"unsafe"

	"github.com/logicossoftware/go-mdocx"
)

func main() {}

// MdocxVersion returns the MDOCX format version supported by this library.
//
//export MdocxVersion
func MdocxVersion() C.uint16_t {
	return C.uint16_t(mdocx.VersionV1)
}

// MdocxFreeResult frees memory allocated by other Mdocx functions.
// Must be called to avoid memory leaks.
//
//export MdocxFreeResult
func MdocxFreeResult(result C.MdocxResult) {
	if result.data != nil {
		C.free(unsafe.Pointer(result.data))
	}
	if result.error != nil {
		C.free(unsafe.Pointer(result.error))
	}
}

// MdocxFreeString frees a C string allocated by Go.
//
//export MdocxFreeString
func MdocxFreeString(s *C.char) {
	if s != nil {
		C.free(unsafe.Pointer(s))
	}
}

// makeResult creates a result with data.
func makeResult(data []byte) C.MdocxResult {
	var result C.MdocxResult
	if len(data) > 0 {
		result.data = (*C.char)(C.CBytes(data))
		result.data_len = C.int(len(data))
	}
	return result
}

// makeError creates a result with an error message.
func makeError(err error) C.MdocxResult {
	var result C.MdocxResult
	result.error = C.CString(err.Error())
	return result
}

// MdocxEncode encodes a document to MDOCX format.
// Parameters:
//   - metadataJSON: optional JSON string for metadata (can be NULL)
//   - markdownFiles: array of CMarkdownFile structs
//   - markdownCount: number of markdown files
//   - mediaItems: array of CMediaItem structs (can be NULL)
//   - mediaCount: number of media items
//   - compression: compression algorithm (0=None, 1=ZIP, 2=ZSTD, 3=LZ4, 4=Brotli)
//
// Returns MdocxResult with encoded data or error. Call MdocxFreeResult when done.
//
//export MdocxEncode
func MdocxEncode(
	metadataJSON *C.char,
	markdownFiles *C.CMarkdownFile,
	markdownCount C.int,
	mediaItems *C.CMediaItem,
	mediaCount C.int,
	compression C.uint16_t,
) C.MdocxResult {
	doc := &mdocx.Document{
		Markdown: mdocx.MarkdownBundle{
			BundleVersion: mdocx.VersionV1,
			Files:         make([]mdocx.MarkdownFile, int(markdownCount)),
		},
		Media: mdocx.MediaBundle{
			BundleVersion: mdocx.VersionV1,
			Items:         make([]mdocx.MediaItem, int(mediaCount)),
		},
	}

	// Parse metadata if provided
	if metadataJSON != nil {
		jsonStr := C.GoString(metadataJSON)
		if jsonStr != "" {
			var meta map[string]any
			if err := json.Unmarshal([]byte(jsonStr), &meta); err != nil {
				return makeError(err)
			}
			doc.Metadata = meta
		}
	}

	// Convert markdown files
	if markdownCount > 0 && markdownFiles != nil {
		mdSlice := unsafe.Slice(markdownFiles, int(markdownCount))
		for i, f := range mdSlice {
			doc.Markdown.Files[i] = mdocx.MarkdownFile{
				Path:    C.GoString(f.path),
				Content: C.GoBytes(unsafe.Pointer(f.content), f.content_len),
			}
		}
	}

	// Convert media items
	if mediaCount > 0 && mediaItems != nil {
		mediaSlice := unsafe.Slice(mediaItems, int(mediaCount))
		for i, m := range mediaSlice {
			doc.Media.Items[i] = mdocx.MediaItem{
				ID:       C.GoString(m.id),
				Path:     C.GoString(m.path),
				MIMEType: C.GoString(m.mime_type),
				Data:     C.GoBytes(unsafe.Pointer(m.data), m.data_len),
			}
		}
	}

	var buf bytes.Buffer
	comp := mdocx.Compression(compression)
	err := mdocx.Encode(&buf, doc,
		mdocx.WithMarkdownCompression(comp),
		mdocx.WithMediaCompression(comp),
	)
	if err != nil {
		return makeError(err)
	}

	return makeResult(buf.Bytes())
}

// MdocxDecode decodes an MDOCX file and returns JSON representation of the document.
// Parameters:
//   - data: pointer to MDOCX file bytes
//   - dataLen: length of the data
//
// Returns MdocxResult with JSON string or error. Call MdocxFreeResult when done.
// The JSON structure contains: metadata, markdown (with files array), media (with items array).
//
//export MdocxDecode
func MdocxDecode(data *C.char, dataLen C.int) C.MdocxResult {
	goData := C.GoBytes(unsafe.Pointer(data), dataLen)
	reader := bytes.NewReader(goData)

	doc, err := mdocx.Decode(reader)
	if err != nil {
		return makeError(err)
	}

	// Convert to JSON-serializable format
	result := map[string]any{
		"metadata": doc.Metadata,
		"markdown": map[string]any{
			"bundleVersion": doc.Markdown.BundleVersion,
			"rootPath":      doc.Markdown.RootPath,
			"files":         make([]map[string]any, len(doc.Markdown.Files)),
		},
		"media": map[string]any{
			"bundleVersion": doc.Media.BundleVersion,
			"items":         make([]map[string]any, len(doc.Media.Items)),
		},
	}

	mdResult := result["markdown"].(map[string]any)
	files := mdResult["files"].([]map[string]any)
	for i, f := range doc.Markdown.Files {
		files[i] = map[string]any{
			"path":       f.Path,
			"content":    string(f.Content),
			"mediaRefs":  f.MediaRefs,
			"attributes": f.Attributes,
		}
	}

	mediaResult := result["media"].(map[string]any)
	items := mediaResult["items"].([]map[string]any)
	for i, m := range doc.Media.Items {
		items[i] = map[string]any{
			"id":         m.ID,
			"path":       m.Path,
			"mimeType":   m.MIMEType,
			"dataLen":    len(m.Data),
			"attributes": m.Attributes,
		}
	}

	jsonBytes, err := json.Marshal(result)
	if err != nil {
		return makeError(err)
	}

	return makeResult(jsonBytes)
}

// MdocxDecodeGetMediaData retrieves the raw data for a specific media item by ID.
// Parameters:
//   - data: pointer to MDOCX file bytes
//   - dataLen: length of the data
//   - mediaID: the ID of the media item to retrieve
//
// Returns MdocxResult with media data or error. Call MdocxFreeResult when done.
//
//export MdocxDecodeGetMediaData
func MdocxDecodeGetMediaData(data *C.char, dataLen C.int, mediaID *C.char) C.MdocxResult {
	goData := C.GoBytes(unsafe.Pointer(data), dataLen)
	reader := bytes.NewReader(goData)
	id := C.GoString(mediaID)

	doc, err := mdocx.Decode(reader)
	if err != nil {
		return makeError(err)
	}

	for _, item := range doc.Media.Items {
		if item.ID == id {
			return makeResult(item.Data)
		}
	}

	var result C.MdocxResult
	result.error = C.CString("media item not found: " + id)
	return result
}

// MdocxValidate validates an MDOCX file without fully parsing it.
// Returns NULL on success, or an error message string on failure.
// Call MdocxFreeString on the result if non-NULL.
//
//export MdocxValidate
func MdocxValidate(data *C.char, dataLen C.int) *C.char {
	goData := C.GoBytes(unsafe.Pointer(data), dataLen)
	reader := bytes.NewReader(goData)

	_, err := mdocx.Decode(reader)
	if err != nil {
		return C.CString(err.Error())
	}
	return nil
}

// MdocxEncodeSimple is a simplified encode function for single-file documents.
// Parameters:
//   - title: optional document title (can be NULL)
//   - markdownPath: path for the markdown file (e.g., "readme.md")
//   - markdownContent: the markdown content
//   - markdownLen: length of markdown content
//
// Returns MdocxResult with encoded data or error. Call MdocxFreeResult when done.
//
//export MdocxEncodeSimple
func MdocxEncodeSimple(
	title *C.char,
	markdownPath *C.char,
	markdownContent *C.char,
	markdownLen C.int,
) C.MdocxResult {
	doc := &mdocx.Document{
		Markdown: mdocx.MarkdownBundle{
			BundleVersion: mdocx.VersionV1,
			Files: []mdocx.MarkdownFile{
				{
					Path:    C.GoString(markdownPath),
					Content: C.GoBytes(unsafe.Pointer(markdownContent), markdownLen),
				},
			},
		},
		Media: mdocx.MediaBundle{BundleVersion: mdocx.VersionV1},
	}

	if title != nil {
		titleStr := C.GoString(title)
		if titleStr != "" {
			doc.Metadata = map[string]any{"title": titleStr}
		}
	}

	var buf bytes.Buffer
	if err := mdocx.Encode(&buf, doc); err != nil {
		return makeError(err)
	}

	return makeResult(buf.Bytes())
}

// MdocxGetMarkdownCount returns the number of markdown files in an MDOCX document.
// Returns -1 on error.
//
//export MdocxGetMarkdownCount
func MdocxGetMarkdownCount(data *C.char, dataLen C.int) C.int {
	goData := C.GoBytes(unsafe.Pointer(data), dataLen)
	reader := bytes.NewReader(goData)

	doc, err := mdocx.Decode(reader)
	if err != nil {
		return -1
	}
	return C.int(len(doc.Markdown.Files))
}

// MdocxGetMediaCount returns the number of media items in an MDOCX document.
// Returns -1 on error.
//
//export MdocxGetMediaCount
func MdocxGetMediaCount(data *C.char, dataLen C.int) C.int {
	goData := C.GoBytes(unsafe.Pointer(data), dataLen)
	reader := bytes.NewReader(goData)

	doc, err := mdocx.Decode(reader)
	if err != nil {
		return -1
	}
	return C.int(len(doc.Media.Items))
}
