package main

import (
	"crypto/sha256"
	"encoding/hex"
	"encoding/json"
	"flag"
	"fmt"
	"log"
	"os"
	"path/filepath"

	"github.com/logicossoftware/go-mdocx"
)

// ValidationResult is the JSON output structure for validation results.
type ValidationResult struct {
	Valid   bool        `json:"valid"`
	Error   string      `json:"error,omitempty"`
	Header  *HeaderInfo `json:"header,omitempty"`
	Summary *DocSummary `json:"summary,omitempty"`
	Details *DocDetails `json:"details,omitempty"`
}

// HeaderInfo contains fixed header information.
type HeaderInfo struct {
	MagicHex       string `json:"magic_hex"`
	MagicValid     bool   `json:"magic_valid"`
	Version        uint16 `json:"version"`
	HeaderFlags    uint16 `json:"header_flags"`
	FixedHdrSize   uint32 `json:"fixed_header_size"`
	MetadataLength uint32 `json:"metadata_length"`
}

// DocSummary provides a high-level summary of the document.
type DocSummary struct {
	HasMetadata        bool `json:"has_metadata"`
	MarkdownFileCount  int  `json:"markdown_file_count"`
	MediaItemCount     int  `json:"media_item_count"`
	TotalMarkdownBytes int  `json:"total_markdown_bytes"`
	TotalMediaBytes    int  `json:"total_media_bytes"`
}

// DocDetails provides detailed information about the document contents.
type DocDetails struct {
	Metadata      map[string]any  `json:"metadata,omitempty"`
	MarkdownFiles []MarkdownInfo  `json:"markdown_files"`
	MediaItems    []MediaItemInfo `json:"media_items"`
}

// MarkdownInfo describes a single markdown file.
type MarkdownInfo struct {
	Path           string            `json:"path"`
	ContentLength  int               `json:"content_length"`
	ContentSHA256  string            `json:"content_sha256"`
	MediaRefs      []string          `json:"media_refs,omitempty"`
	Attributes     map[string]string `json:"attributes,omitempty"`
	ContentPreview string            `json:"content_preview,omitempty"`
}

// MediaItemInfo describes a single media item.
type MediaItemInfo struct {
	ID             string            `json:"id"`
	Path           string            `json:"path,omitempty"`
	MIMEType       string            `json:"mime_type,omitempty"`
	DataLength     int               `json:"data_length"`
	SHA256Stored   string            `json:"sha256_stored,omitempty"`
	SHA256Computed string            `json:"sha256_computed"`
	SHA256Valid    bool              `json:"sha256_valid"`
	Attributes     map[string]string `json:"attributes,omitempty"`
}

// TestSuiteManifest describes all generated test files.
type TestSuiteManifest struct {
	Description string         `json:"description"`
	Files       []TestFileInfo `json:"files"`
}

// TestFileInfo describes a single test file.
type TestFileInfo struct {
	Filename    string `json:"filename"`
	Description string `json:"description"`
	Compression string `json:"compression"`
	HasMetadata bool   `json:"has_metadata"`
	HasMedia    bool   `json:"has_media"`
	FileCount   int    `json:"markdown_file_count"`
	MediaCount  int    `json:"media_item_count"`
}

func main() {
	var inPath string
	var includeDetails bool
	var includePreview bool
	var previewLen int
	var generateTestSuite string

	flag.StringVar(&inPath, "in", "", "input .mdocx file to validate")
	flag.BoolVar(&includeDetails, "details", false, "include detailed file/media information")
	flag.BoolVar(&includePreview, "preview", false, "include content preview for markdown files (requires -details)")
	flag.IntVar(&previewLen, "preview-len", 200, "maximum length of content preview")
	flag.StringVar(&generateTestSuite, "generate-test-suite", "", "generate test suite files in specified directory")
	flag.Parse()

	// Generate test suite mode
	if generateTestSuite != "" {
		if err := generateTestFiles(generateTestSuite); err != nil {
			log.Fatalf("failed to generate test suite: %v", err)
		}
		return
	}

	// Validation mode
	if inPath == "" {
		result := ValidationResult{Valid: false, Error: "missing -in flag: input file path required"}
		outputJSON(result)
		os.Exit(1)
	}

	result := validateFile(inPath, includeDetails, includePreview, previewLen)
	outputJSON(result)

	if !result.Valid {
		os.Exit(1)
	}
}

func outputJSON(v any) {
	enc := json.NewEncoder(os.Stdout)
	enc.SetIndent("", "  ")
	if err := enc.Encode(v); err != nil {
		log.Fatalf("failed to encode JSON: %v", err)
	}
}

func validateFile(path string, includeDetails, includePreview bool, previewLen int) ValidationResult {
	f, err := os.Open(path)
	if err != nil {
		return ValidationResult{Valid: false, Error: fmt.Sprintf("failed to open file: %v", err)}
	}
	defer f.Close()

	// Read raw header for reporting
	headerInfo, headerErr := readRawHeader(path)

	doc, err := mdocx.Decode(f)
	if err != nil {
		result := ValidationResult{
			Valid:  false,
			Error:  fmt.Sprintf("decode failed: %v", err),
			Header: headerInfo,
		}
		return result
	}

	if headerErr != nil {
		// This shouldn't happen if Decode succeeded, but handle gracefully
		headerInfo = nil
	}

	summary := &DocSummary{
		HasMetadata:       doc.Metadata != nil,
		MarkdownFileCount: len(doc.Markdown.Files),
		MediaItemCount:    len(doc.Media.Items),
	}

	for _, mf := range doc.Markdown.Files {
		summary.TotalMarkdownBytes += len(mf.Content)
	}
	for _, mi := range doc.Media.Items {
		summary.TotalMediaBytes += len(mi.Data)
	}

	result := ValidationResult{
		Valid:   true,
		Header:  headerInfo,
		Summary: summary,
	}

	if includeDetails {
		result.Details = buildDetails(doc, includePreview, previewLen)
	}

	return result
}

func readRawHeader(path string) (*HeaderInfo, error) {
	f, err := os.Open(path)
	if err != nil {
		return nil, err
	}
	defer f.Close()

	var buf [32]byte
	n, err := f.Read(buf[:])
	if err != nil || n < 32 {
		return nil, fmt.Errorf("failed to read header")
	}

	expectedMagic := [8]byte{'M', 'D', 'O', 'C', 'X', '\r', '\n', 0x1A}
	var actualMagic [8]byte
	copy(actualMagic[:], buf[0:8])
	magicValid := actualMagic == expectedMagic

	return &HeaderInfo{
		MagicHex:       hex.EncodeToString(buf[0:8]),
		MagicValid:     magicValid,
		Version:        uint16(buf[8]) | uint16(buf[9])<<8,
		HeaderFlags:    uint16(buf[10]) | uint16(buf[11])<<8,
		FixedHdrSize:   uint32(buf[12]) | uint32(buf[13])<<8 | uint32(buf[14])<<16 | uint32(buf[15])<<24,
		MetadataLength: uint32(buf[16]) | uint32(buf[17])<<8 | uint32(buf[18])<<16 | uint32(buf[19])<<24,
	}, nil
}

func buildDetails(doc *mdocx.Document, includePreview bool, previewLen int) *DocDetails {
	details := &DocDetails{
		Metadata:      doc.Metadata,
		MarkdownFiles: make([]MarkdownInfo, 0, len(doc.Markdown.Files)),
		MediaItems:    make([]MediaItemInfo, 0, len(doc.Media.Items)),
	}

	for _, mf := range doc.Markdown.Files {
		h := sha256.Sum256(mf.Content)
		info := MarkdownInfo{
			Path:          mf.Path,
			ContentLength: len(mf.Content),
			ContentSHA256: hex.EncodeToString(h[:]),
			MediaRefs:     mf.MediaRefs,
			Attributes:    mf.Attributes,
		}
		if includePreview && len(mf.Content) > 0 {
			preview := string(mf.Content)
			if len(preview) > previewLen {
				preview = preview[:previewLen] + "..."
			}
			info.ContentPreview = preview
		}
		details.MarkdownFiles = append(details.MarkdownFiles, info)
	}

	for _, mi := range doc.Media.Items {
		computed := sha256.Sum256(mi.Data)
		storedHex := ""
		if mi.SHA256 != ([32]byte{}) {
			storedHex = hex.EncodeToString(mi.SHA256[:])
		}
		sha256Valid := true
		if mi.SHA256 != ([32]byte{}) {
			sha256Valid = mi.SHA256 == computed
		}

		info := MediaItemInfo{
			ID:             mi.ID,
			Path:           mi.Path,
			MIMEType:       mi.MIMEType,
			DataLength:     len(mi.Data),
			SHA256Stored:   storedHex,
			SHA256Computed: hex.EncodeToString(computed[:]),
			SHA256Valid:    sha256Valid,
			Attributes:     mi.Attributes,
		}
		details.MediaItems = append(details.MediaItems, info)
	}

	return details
}

// generateTestFiles creates a comprehensive test suite for cross-language testing.
func generateTestFiles(outDir string) error {
	if err := os.MkdirAll(outDir, 0o755); err != nil {
		return fmt.Errorf("create output directory: %w", err)
	}

	manifest := TestSuiteManifest{
		Description: "MDOCX v1 test suite for cross-language implementation testing",
		Files:       make([]TestFileInfo, 0),
	}

	// Test case generators
	testCases := []struct {
		name        string
		description string
		generate    func() (*mdocx.Document, mdocx.Compression, mdocx.Compression)
	}{
		{
			name:        "minimal_uncompressed.mdocx",
			description: "Minimal valid file with no compression, no metadata, no media",
			generate:    generateMinimalUncompressed,
		},
		{
			name:        "minimal_zstd.mdocx",
			description: "Minimal valid file with ZSTD compression",
			generate:    generateMinimalZSTD,
		},
		{
			name:        "minimal_zip.mdocx",
			description: "Minimal valid file with ZIP compression",
			generate:    generateMinimalZIP,
		},
		{
			name:        "minimal_lz4.mdocx",
			description: "Minimal valid file with LZ4 compression",
			generate:    generateMinimalLZ4,
		},
		{
			name:        "minimal_brotli.mdocx",
			description: "Minimal valid file with Brotli compression",
			generate:    generateMinimalBrotli,
		},
		{
			name:        "with_metadata.mdocx",
			description: "File with full metadata block",
			generate:    generateWithMetadata,
		},
		{
			name:        "multi_markdown.mdocx",
			description: "Multiple markdown files with cross-references",
			generate:    generateMultiMarkdown,
		},
		{
			name:        "with_media.mdocx",
			description: "File with media items including SHA256 hashes",
			generate:    generateWithMedia,
		},
		{
			name:        "full_featured.mdocx",
			description: "Full-featured file with metadata, multiple markdown files, media, attributes",
			generate:    generateFullFeatured,
		},
		{
			name:        "media_refs.mdocx",
			description: "Markdown with media references using mdocx:// URIs",
			generate:    generateMediaRefs,
		},
		{
			name:        "unicode_content.mdocx",
			description: "Unicode content in markdown and metadata",
			generate:    generateUnicodeContent,
		},
		{
			name:        "empty_media_bundle.mdocx",
			description: "Valid file with explicitly empty media bundle",
			generate:    generateEmptyMediaBundle,
		},
		{
			name:        "attributes.mdocx",
			description: "Files and media with custom attributes",
			generate:    generateWithAttributes,
		},
		{
			name:        "deep_paths.mdocx",
			description: "Deeply nested file paths",
			generate:    generateDeepPaths,
		},
		{
			name:        "large_content.mdocx",
			description: "Larger content to test compression effectiveness",
			generate:    generateLargeContent,
		},
	}

	for _, tc := range testCases {
		doc, mdComp, mediaComp := tc.generate()

		filePath := filepath.Join(outDir, tc.name)
		f, err := os.Create(filePath)
		if err != nil {
			return fmt.Errorf("create %s: %w", tc.name, err)
		}

		err = mdocx.Encode(f, doc,
			mdocx.WithMarkdownCompression(mdComp),
			mdocx.WithMediaCompression(mediaComp),
		)
		f.Close()
		if err != nil {
			return fmt.Errorf("encode %s: %w", tc.name, err)
		}

		// Generate expected output JSON
		jsonPath := filepath.Join(outDir, tc.name+".expected.json")
		result := validateFile(filePath, true, true, 500)
		jsonFile, err := os.Create(jsonPath)
		if err != nil {
			return fmt.Errorf("create %s: %w", jsonPath, err)
		}
		enc := json.NewEncoder(jsonFile)
		enc.SetIndent("", "  ")
		err = enc.Encode(result)
		jsonFile.Close()
		if err != nil {
			return fmt.Errorf("write %s: %w", jsonPath, err)
		}

		manifest.Files = append(manifest.Files, TestFileInfo{
			Filename:    tc.name,
			Description: tc.description,
			Compression: compressionName(mdComp),
			HasMetadata: doc.Metadata != nil,
			HasMedia:    len(doc.Media.Items) > 0,
			FileCount:   len(doc.Markdown.Files),
			MediaCount:  len(doc.Media.Items),
		})

		fmt.Printf("Generated: %s\n", tc.name)
	}

	// Write manifest
	manifestPath := filepath.Join(outDir, "manifest.json")
	mf, err := os.Create(manifestPath)
	if err != nil {
		return fmt.Errorf("create manifest: %w", err)
	}
	defer mf.Close()
	enc := json.NewEncoder(mf)
	enc.SetIndent("", "  ")
	if err := enc.Encode(manifest); err != nil {
		return fmt.Errorf("write manifest: %w", err)
	}

	fmt.Printf("\nGenerated %d test files in %s\n", len(manifest.Files), outDir)
	fmt.Printf("Manifest: %s\n", manifestPath)
	return nil
}

func compressionName(c mdocx.Compression) string {
	switch c {
	case mdocx.CompNone:
		return "none"
	case mdocx.CompZIP:
		return "zip"
	case mdocx.CompZSTD:
		return "zstd"
	case mdocx.CompLZ4:
		return "lz4"
	case mdocx.CompBR:
		return "brotli"
	default:
		return "unknown"
	}
}

// --- Test case generators ---

func generateMinimalUncompressed() (*mdocx.Document, mdocx.Compression, mdocx.Compression) {
	doc := &mdocx.Document{
		Markdown: mdocx.MarkdownBundle{
			BundleVersion: mdocx.VersionV1,
			Files: []mdocx.MarkdownFile{
				{Path: "readme.md", Content: []byte("# Minimal\n")},
			},
		},
		Media: mdocx.MediaBundle{BundleVersion: mdocx.VersionV1},
	}
	return doc, mdocx.CompNone, mdocx.CompNone
}

func generateMinimalZSTD() (*mdocx.Document, mdocx.Compression, mdocx.Compression) {
	doc := &mdocx.Document{
		Markdown: mdocx.MarkdownBundle{
			BundleVersion: mdocx.VersionV1,
			Files: []mdocx.MarkdownFile{
				{Path: "readme.md", Content: []byte("# ZSTD Compressed\n")},
			},
		},
		Media: mdocx.MediaBundle{BundleVersion: mdocx.VersionV1},
	}
	return doc, mdocx.CompZSTD, mdocx.CompZSTD
}

func generateMinimalZIP() (*mdocx.Document, mdocx.Compression, mdocx.Compression) {
	doc := &mdocx.Document{
		Markdown: mdocx.MarkdownBundle{
			BundleVersion: mdocx.VersionV1,
			Files: []mdocx.MarkdownFile{
				{Path: "readme.md", Content: []byte("# ZIP Compressed\n")},
			},
		},
		Media: mdocx.MediaBundle{BundleVersion: mdocx.VersionV1},
	}
	return doc, mdocx.CompZIP, mdocx.CompZIP
}

func generateMinimalLZ4() (*mdocx.Document, mdocx.Compression, mdocx.Compression) {
	doc := &mdocx.Document{
		Markdown: mdocx.MarkdownBundle{
			BundleVersion: mdocx.VersionV1,
			Files: []mdocx.MarkdownFile{
				{Path: "readme.md", Content: []byte("# LZ4 Compressed\n")},
			},
		},
		Media: mdocx.MediaBundle{BundleVersion: mdocx.VersionV1},
	}
	return doc, mdocx.CompLZ4, mdocx.CompLZ4
}

func generateMinimalBrotli() (*mdocx.Document, mdocx.Compression, mdocx.Compression) {
	doc := &mdocx.Document{
		Markdown: mdocx.MarkdownBundle{
			BundleVersion: mdocx.VersionV1,
			Files: []mdocx.MarkdownFile{
				{Path: "readme.md", Content: []byte("# Brotli Compressed\n")},
			},
		},
		Media: mdocx.MediaBundle{BundleVersion: mdocx.VersionV1},
	}
	return doc, mdocx.CompBR, mdocx.CompBR
}

func generateWithMetadata() (*mdocx.Document, mdocx.Compression, mdocx.Compression) {
	doc := &mdocx.Document{
		Metadata: map[string]any{
			"title":       "Test Document",
			"description": "A document for testing metadata parsing",
			"creator":     "MDOCX Test Suite",
			"created_at":  "2026-01-05T00:00:00Z",
			"root":        "docs/index.md",
			"tags":        []any{"test", "mdocx", "validation"},
			"version":     1.0,
			"draft":       false,
		},
		Markdown: mdocx.MarkdownBundle{
			BundleVersion: mdocx.VersionV1,
			RootPath:      "docs/index.md",
			Files: []mdocx.MarkdownFile{
				{Path: "docs/index.md", Content: []byte("# Document with Metadata\n\nThis file tests metadata parsing.\n")},
			},
		},
		Media: mdocx.MediaBundle{BundleVersion: mdocx.VersionV1},
	}
	return doc, mdocx.CompZSTD, mdocx.CompZSTD
}

func generateMultiMarkdown() (*mdocx.Document, mdocx.Compression, mdocx.Compression) {
	doc := &mdocx.Document{
		Metadata: map[string]any{
			"title": "Multi-file Document",
			"root":  "index.md",
		},
		Markdown: mdocx.MarkdownBundle{
			BundleVersion: mdocx.VersionV1,
			RootPath:      "index.md",
			Files: []mdocx.MarkdownFile{
				{Path: "index.md", Content: []byte("# Main Document\n\n- [Chapter 1](chapters/ch1.md)\n- [Chapter 2](chapters/ch2.md)\n- [Appendix](appendix/a.md)\n")},
				{Path: "chapters/ch1.md", Content: []byte("# Chapter 1\n\nFirst chapter content.\n\n[Back to index](../index.md)\n")},
				{Path: "chapters/ch2.md", Content: []byte("# Chapter 2\n\nSecond chapter content.\n\n[Back to index](../index.md)\n")},
				{Path: "appendix/a.md", Content: []byte("# Appendix A\n\nAdditional information.\n")},
			},
		},
		Media: mdocx.MediaBundle{BundleVersion: mdocx.VersionV1},
	}
	return doc, mdocx.CompZSTD, mdocx.CompZSTD
}

func generateWithMedia() (*mdocx.Document, mdocx.Compression, mdocx.Compression) {
	// Create sample binary data for different media types
	pngData := []byte{0x89, 'P', 'N', 'G', 0x0D, 0x0A, 0x1A, 0x0A, 0x00, 0x00, 0x00, 0x0D, 'I', 'H', 'D', 'R'}
	jpgData := []byte{0xFF, 0xD8, 0xFF, 0xE0, 0x00, 0x10, 'J', 'F', 'I', 'F', 0x00, 0x01, 0x01, 0x00, 0x00, 0x01}
	txtData := []byte("This is a plain text attachment.\n")

	pngHash := sha256.Sum256(pngData)
	jpgHash := sha256.Sum256(jpgData)
	txtHash := sha256.Sum256(txtData)

	doc := &mdocx.Document{
		Metadata: map[string]any{
			"title": "Document with Media",
		},
		Markdown: mdocx.MarkdownBundle{
			BundleVersion: mdocx.VersionV1,
			Files: []mdocx.MarkdownFile{
				{
					Path:      "readme.md",
					Content:   []byte("# Document with Media\n\n![Logo](assets/logo.png)\n![Photo](assets/photo.jpg)\n"),
					MediaRefs: []string{"logo", "photo"},
				},
			},
		},
		Media: mdocx.MediaBundle{
			BundleVersion: mdocx.VersionV1,
			Items: []mdocx.MediaItem{
				{ID: "logo", Path: "assets/logo.png", MIMEType: "image/png", Data: pngData, SHA256: pngHash},
				{ID: "photo", Path: "assets/photo.jpg", MIMEType: "image/jpeg", Data: jpgData, SHA256: jpgHash},
				{ID: "notes", Path: "attachments/notes.txt", MIMEType: "text/plain", Data: txtData, SHA256: txtHash},
			},
		},
	}
	return doc, mdocx.CompZSTD, mdocx.CompZSTD
}

func generateFullFeatured() (*mdocx.Document, mdocx.Compression, mdocx.Compression) {
	imgData := []byte{0x89, 'P', 'N', 'G', 0x0D, 0x0A, 0x1A, 0x0A, 1, 2, 3, 4, 5, 6, 7, 8}
	audioData := []byte{'I', 'D', '3', 0x04, 0x00, 0x00, 0x00, 0x00, 0x00, 0x00}
	imgHash := sha256.Sum256(imgData)
	audioHash := sha256.Sum256(audioData)

	doc := &mdocx.Document{
		Metadata: map[string]any{
			"title":       "Full Featured MDOCX",
			"description": "Comprehensive test file with all features",
			"creator":     "MDOCX Test Suite Generator",
			"created_at":  "2026-01-05T12:00:00Z",
			"root":        "docs/index.md",
			"tags":        []any{"full", "test", "comprehensive"},
			"custom": map[string]any{
				"nested": true,
				"count":  42,
			},
		},
		Markdown: mdocx.MarkdownBundle{
			BundleVersion: mdocx.VersionV1,
			RootPath:      "docs/index.md",
			Files: []mdocx.MarkdownFile{
				{
					Path:       "docs/index.md",
					Content:    []byte("# Full Featured Document\n\n![Banner](mdocx://media/banner)\n\n## Contents\n\n- [Guide](guide.md)\n- [Reference](reference.md)\n"),
					MediaRefs:  []string{"banner"},
					Attributes: map[string]string{"language": "en", "status": "final"},
				},
				{
					Path:       "docs/guide.md",
					Content:    []byte("# User Guide\n\nThis is the user guide.\n\n🎵 [Listen](mdocx://media/audio_sample)\n"),
					MediaRefs:  []string{"audio_sample"},
					Attributes: map[string]string{"language": "en", "chapter": "1"},
				},
				{
					Path:       "docs/reference.md",
					Content:    []byte("# API Reference\n\n```go\nfunc Example() {}\n```\n"),
					Attributes: map[string]string{"language": "en", "chapter": "2"},
				},
			},
		},
		Media: mdocx.MediaBundle{
			BundleVersion: mdocx.VersionV1,
			Items: []mdocx.MediaItem{
				{
					ID:         "banner",
					Path:       "media/banner.png",
					MIMEType:   "image/png",
					Data:       imgData,
					SHA256:     imgHash,
					Attributes: map[string]string{"alt": "Document Banner", "width": "800", "height": "200"},
				},
				{
					ID:         "audio_sample",
					Path:       "media/sample.mp3",
					MIMEType:   "audio/mpeg",
					Data:       audioData,
					SHA256:     audioHash,
					Attributes: map[string]string{"duration": "3.5", "title": "Sample Audio"},
				},
			},
		},
	}
	return doc, mdocx.CompZSTD, mdocx.CompZSTD
}

func generateMediaRefs() (*mdocx.Document, mdocx.Compression, mdocx.Compression) {
	img1 := []byte{1, 2, 3, 4, 5}
	img2 := []byte{6, 7, 8, 9, 10}
	hash1 := sha256.Sum256(img1)
	hash2 := sha256.Sum256(img2)

	doc := &mdocx.Document{
		Markdown: mdocx.MarkdownBundle{
			BundleVersion: mdocx.VersionV1,
			Files: []mdocx.MarkdownFile{
				{
					Path: "readme.md",
					Content: []byte(`# Media References Test

## Using mdocx:// URIs
![Image 1](mdocx://media/img1)
![Image 2](mdocx://media/img2)

## Using relative paths
![Image 1](assets/image1.png)
![Image 2](assets/image2.png)
`),
					MediaRefs: []string{"img1", "img2"},
				},
			},
		},
		Media: mdocx.MediaBundle{
			BundleVersion: mdocx.VersionV1,
			Items: []mdocx.MediaItem{
				{ID: "img1", Path: "assets/image1.png", MIMEType: "image/png", Data: img1, SHA256: hash1},
				{ID: "img2", Path: "assets/image2.png", MIMEType: "image/png", Data: img2, SHA256: hash2},
			},
		},
	}
	return doc, mdocx.CompZSTD, mdocx.CompZSTD
}

func generateUnicodeContent() (*mdocx.Document, mdocx.Compression, mdocx.Compression) {
	doc := &mdocx.Document{
		Metadata: map[string]any{
			"title":       "Unicode Test: 日本語 中文 한국어",
			"description": "Testing UTF-8 content: émojis 🎉🚀💻, symbols ∑∏∫, accents éàü",
			"tags":        []any{"测试", "テスト", "시험"},
		},
		Markdown: mdocx.MarkdownBundle{
			BundleVersion: mdocx.VersionV1,
			Files: []mdocx.MarkdownFile{
				{
					Path: "unicode.md",
					Content: []byte(`# Unicode Content Test

## Emojis
🎉 Party! 🚀 Rocket! 💻 Computer!

## CJK Characters
- 日本語: これはテストです
- 中文: 这是一个测试
- 한국어: 이것은 테스트입니다

## European Characters
- French: Ça c'est génial!
- German: Größe und Übung
- Spanish: ¡Hola! ¿Cómo estás?

## Mathematical Symbols
∑ ∏ ∫ ∂ ∇ √ ∞ ≈ ≠ ≤ ≥

## Currency
$ € £ ¥ ₹ ₽ ₿
`),
				},
			},
		},
		Media: mdocx.MediaBundle{BundleVersion: mdocx.VersionV1},
	}
	return doc, mdocx.CompZSTD, mdocx.CompZSTD
}

func generateEmptyMediaBundle() (*mdocx.Document, mdocx.Compression, mdocx.Compression) {
	doc := &mdocx.Document{
		Metadata: map[string]any{
			"title": "Empty Media Bundle Test",
		},
		Markdown: mdocx.MarkdownBundle{
			BundleVersion: mdocx.VersionV1,
			Files: []mdocx.MarkdownFile{
				{Path: "readme.md", Content: []byte("# No Media\n\nThis document has no media items.\n")},
			},
		},
		Media: mdocx.MediaBundle{
			BundleVersion: mdocx.VersionV1,
			Items:         []mdocx.MediaItem{}, // Explicitly empty
		},
	}
	return doc, mdocx.CompZSTD, mdocx.CompZSTD
}

func generateWithAttributes() (*mdocx.Document, mdocx.Compression, mdocx.Compression) {
	data := []byte{0xDE, 0xAD, 0xBE, 0xEF}
	hash := sha256.Sum256(data)

	doc := &mdocx.Document{
		Metadata: map[string]any{
			"title": "Attributes Test",
		},
		Markdown: mdocx.MarkdownBundle{
			BundleVersion: mdocx.VersionV1,
			Files: []mdocx.MarkdownFile{
				{
					Path:    "doc1.md",
					Content: []byte("# Document 1\n"),
					Attributes: map[string]string{
						"author":   "Alice",
						"language": "en",
						"status":   "draft",
						"priority": "high",
					},
				},
				{
					Path:    "doc2.md",
					Content: []byte("# Document 2\n"),
					Attributes: map[string]string{
						"author":   "Bob",
						"language": "de",
						"status":   "final",
					},
				},
			},
		},
		Media: mdocx.MediaBundle{
			BundleVersion: mdocx.VersionV1,
			Items: []mdocx.MediaItem{
				{
					ID:       "data",
					Path:     "data.bin",
					MIMEType: "application/octet-stream",
					Data:     data,
					SHA256:   hash,
					Attributes: map[string]string{
						"encoding":    "binary",
						"compression": "none",
						"checksum":    "deadbeef",
					},
				},
			},
		},
	}
	return doc, mdocx.CompZSTD, mdocx.CompZSTD
}

func generateDeepPaths() (*mdocx.Document, mdocx.Compression, mdocx.Compression) {
	doc := &mdocx.Document{
		Metadata: map[string]any{
			"title": "Deep Paths Test",
			"root":  "level1/level2/level3/level4/index.md",
		},
		Markdown: mdocx.MarkdownBundle{
			BundleVersion: mdocx.VersionV1,
			RootPath:      "level1/level2/level3/level4/index.md",
			Files: []mdocx.MarkdownFile{
				{Path: "level1/level2/level3/level4/index.md", Content: []byte("# Deep Index\n")},
				{Path: "level1/level2/level3/level4/chapter1.md", Content: []byte("# Chapter 1\n")},
				{Path: "level1/level2/another/path/doc.md", Content: []byte("# Another Doc\n")},
				{Path: "top.md", Content: []byte("# Top Level\n")},
			},
		},
		Media: mdocx.MediaBundle{BundleVersion: mdocx.VersionV1},
	}
	return doc, mdocx.CompZSTD, mdocx.CompZSTD
}

func generateLargeContent() (*mdocx.Document, mdocx.Compression, mdocx.Compression) {
	// Generate repetitive content that compresses well
	var content []byte
	content = append(content, []byte("# Large Content Test\n\n")...)
	for i := 0; i < 100; i++ {
		content = append(content, []byte(fmt.Sprintf("## Section %d\n\nThis is paragraph %d with some repeated content to test compression. ", i+1, i+1))...)
		content = append(content, []byte("Lorem ipsum dolor sit amet, consectetur adipiscing elit. Sed do eiusmod tempor incididunt ut labore et dolore magna aliqua.\n\n")...)
	}

	// Generate binary data
	binaryData := make([]byte, 1024)
	for i := range binaryData {
		binaryData[i] = byte(i % 256)
	}
	binaryHash := sha256.Sum256(binaryData)

	doc := &mdocx.Document{
		Metadata: map[string]any{
			"title":       "Large Content Test",
			"description": "Tests compression with larger payloads",
		},
		Markdown: mdocx.MarkdownBundle{
			BundleVersion: mdocx.VersionV1,
			Files: []mdocx.MarkdownFile{
				{Path: "large.md", Content: content},
			},
		},
		Media: mdocx.MediaBundle{
			BundleVersion: mdocx.VersionV1,
			Items: []mdocx.MediaItem{
				{ID: "binary", Path: "data.bin", MIMEType: "application/octet-stream", Data: binaryData, SHA256: binaryHash},
			},
		},
	}
	return doc, mdocx.CompZSTD, mdocx.CompZSTD
}
