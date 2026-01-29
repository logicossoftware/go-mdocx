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
		// Basic compression variants
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
		// Mixed compression modes
		{
			name:        "mixed_zstd_none.mdocx",
			description: "ZSTD markdown with uncompressed media",
			generate:    generateMixedZSTDNone,
		},
		{
			name:        "mixed_zip_lz4.mdocx",
			description: "ZIP markdown with LZ4 media",
			generate:    generateMixedZIPLZ4,
		},
		{
			name:        "mixed_brotli_zstd.mdocx",
			description: "Brotli markdown with ZSTD media",
			generate:    generateMixedBrotliZSTD,
		},
		// Metadata variations
		{
			name:        "with_metadata.mdocx",
			description: "File with full metadata block",
			generate:    generateWithMetadata,
		},
		{
			name:        "metadata_nested.mdocx",
			description: "Deeply nested metadata structures",
			generate:    generateMetadataNested,
		},
		{
			name:        "metadata_types.mdocx",
			description: "Metadata with various JSON types (null, bool, numbers, arrays)",
			generate:    generateMetadataTypes,
		},
		{
			name:        "metadata_unicode.mdocx",
			description: "Metadata with unicode keys and values",
			generate:    generateMetadataUnicode,
		},
		// Multi-file scenarios
		{
			name:        "multi_markdown.mdocx",
			description: "Multiple markdown files with cross-references",
			generate:    generateMultiMarkdown,
		},
		{
			name:        "many_files.mdocx",
			description: "Many small markdown files (50 files)",
			generate:    generateManyFiles,
		},
		{
			name:        "sibling_dirs.mdocx",
			description: "Files organized in sibling directories",
			generate:    generateSiblingDirs,
		},
		// Media scenarios
		{
			name:        "with_media.mdocx",
			description: "File with media items including SHA256 hashes",
			generate:    generateWithMedia,
		},
		{
			name:        "media_no_sha256.mdocx",
			description: "Media items without SHA256 hashes (auto-populated)",
			generate:    generateMediaNoSHA256,
		},
		{
			name:        "media_mime_types.mdocx",
			description: "Various MIME types for media items",
			generate:    generateMediaMIMETypes,
		},
		{
			name:        "media_binary_patterns.mdocx",
			description: "Media with specific binary patterns (all zeros, all ones, patterns)",
			generate:    generateMediaBinaryPatterns,
		},
		// Full featured
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
		// Unicode and special characters
		{
			name:        "unicode_content.mdocx",
			description: "Unicode content in markdown and metadata",
			generate:    generateUnicodeContent,
		},
		{
			name:        "unicode_paths.mdocx",
			description: "Unicode characters in file paths",
			generate:    generateUnicodePaths,
		},
		{
			name:        "special_markdown.mdocx",
			description: "Markdown with code blocks, tables, and special syntax",
			generate:    generateSpecialMarkdown,
		},
		// Edge cases
		{
			name:        "empty_media_bundle.mdocx",
			description: "Valid file with explicitly empty media bundle",
			generate:    generateEmptyMediaBundle,
		},
		{
			name:        "empty_content.mdocx",
			description: "Markdown files with empty content",
			generate:    generateEmptyContent,
		},
		{
			name:        "whitespace_content.mdocx",
			description: "Content with various whitespace patterns",
			generate:    generateWhitespaceContent,
		},
		{
			name:        "single_char.mdocx",
			description: "Single character markdown content",
			generate:    generateSingleChar,
		},
		// Attributes
		{
			name:        "attributes.mdocx",
			description: "Files and media with custom attributes",
			generate:    generateWithAttributes,
		},
		{
			name:        "attributes_empty.mdocx",
			description: "Files with empty attribute maps",
			generate:    generateAttributesEmpty,
		},
		{
			name:        "attributes_unicode.mdocx",
			description: "Attributes with unicode keys and values",
			generate:    generateAttributesUnicode,
		},
		// Path variations
		{
			name:        "deep_paths.mdocx",
			description: "Deeply nested file paths",
			generate:    generateDeepPaths,
		},
		{
			name:        "dotfile_paths.mdocx",
			description: "Paths with dotfiles and hidden files",
			generate:    generateDotfilePaths,
		},
		{
			name:        "extension_variety.mdocx",
			description: "Various file extensions (.md, .markdown, .txt, .mdx)",
			generate:    generateExtensionVariety,
		},
		// Larger content
		{
			name:        "large_content.mdocx",
			description: "Larger content to test compression effectiveness",
			generate:    generateLargeContent,
		},
		{
			name:        "large_single_file.mdocx",
			description: "Single large markdown file (100KB+)",
			generate:    generateLargeSingleFile,
		},
		{
			name:        "large_media.mdocx",
			description: "Larger media items (multiple KB each)",
			generate:    generateLargeMedia,
		},
		// Realistic examples
		{
			name:        "blog_post.mdocx",
			description: "Realistic blog post with frontmatter-style metadata",
			generate:    generateBlogPost,
		},
		{
			name:        "api_docs.mdocx",
			description: "API documentation with code examples",
			generate:    generateAPIDocs,
		},
		{
			name:        "book_chapter.mdocx",
			description: "Book chapter with table of contents and cross-references",
			generate:    generateBookChapter,
		},
		{
			name:        "readme_project.mdocx",
			description: "Typical project README with badges and sections",
			generate:    generateReadmeProject,
		},
		// Compression stress tests
		{
			name:        "highly_compressible.mdocx",
			description: "Highly repetitive content that compresses very well",
			generate:    generateHighlyCompressible,
		},
		{
			name:        "incompressible.mdocx",
			description: "Random-like content that doesn't compress well",
			generate:    generateIncompressible,
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

// --- Mixed compression generators ---

func generateMixedZSTDNone() (*mdocx.Document, mdocx.Compression, mdocx.Compression) {
	data := []byte{0x89, 'P', 'N', 'G', 0x0D, 0x0A, 0x1A, 0x0A}
	hash := sha256.Sum256(data)
	doc := &mdocx.Document{
		Metadata: map[string]any{"title": "Mixed ZSTD/None"},
		Markdown: mdocx.MarkdownBundle{
			BundleVersion: mdocx.VersionV1,
			Files:         []mdocx.MarkdownFile{{Path: "readme.md", Content: []byte("# Mixed Compression\n\nZSTD markdown, uncompressed media.\n")}},
		},
		Media: mdocx.MediaBundle{
			BundleVersion: mdocx.VersionV1,
			Items:         []mdocx.MediaItem{{ID: "img", Path: "img.png", MIMEType: "image/png", Data: data, SHA256: hash}},
		},
	}
	return doc, mdocx.CompZSTD, mdocx.CompNone
}

func generateMixedZIPLZ4() (*mdocx.Document, mdocx.Compression, mdocx.Compression) {
	data := []byte{0xFF, 0xD8, 0xFF, 0xE0, 0x00, 0x10}
	hash := sha256.Sum256(data)
	doc := &mdocx.Document{
		Metadata: map[string]any{"title": "Mixed ZIP/LZ4"},
		Markdown: mdocx.MarkdownBundle{
			BundleVersion: mdocx.VersionV1,
			Files:         []mdocx.MarkdownFile{{Path: "readme.md", Content: []byte("# Mixed Compression\n\nZIP markdown, LZ4 media.\n")}},
		},
		Media: mdocx.MediaBundle{
			BundleVersion: mdocx.VersionV1,
			Items:         []mdocx.MediaItem{{ID: "img", Path: "img.jpg", MIMEType: "image/jpeg", Data: data, SHA256: hash}},
		},
	}
	return doc, mdocx.CompZIP, mdocx.CompLZ4
}

func generateMixedBrotliZSTD() (*mdocx.Document, mdocx.Compression, mdocx.Compression) {
	data := []byte("Text content for media")
	hash := sha256.Sum256(data)
	doc := &mdocx.Document{
		Metadata: map[string]any{"title": "Mixed Brotli/ZSTD"},
		Markdown: mdocx.MarkdownBundle{
			BundleVersion: mdocx.VersionV1,
			Files:         []mdocx.MarkdownFile{{Path: "readme.md", Content: []byte("# Mixed Compression\n\nBrotli markdown, ZSTD media.\n")}},
		},
		Media: mdocx.MediaBundle{
			BundleVersion: mdocx.VersionV1,
			Items:         []mdocx.MediaItem{{ID: "txt", Path: "data.txt", MIMEType: "text/plain", Data: data, SHA256: hash}},
		},
	}
	return doc, mdocx.CompBR, mdocx.CompZSTD
}

// --- Metadata variation generators ---

func generateMetadataNested() (*mdocx.Document, mdocx.Compression, mdocx.Compression) {
	doc := &mdocx.Document{
		Metadata: map[string]any{
			"title": "Nested Metadata Test",
			"author": map[string]any{
				"name":  "John Doe",
				"email": "john@example.com",
				"social": map[string]any{
					"twitter":  "@johndoe",
					"github":   "johndoe",
					"linkedin": "in/johndoe",
				},
			},
			"publication": map[string]any{
				"date":    "2026-01-07",
				"edition": 1,
				"details": map[string]any{
					"isbn":      "978-0-123456-78-9",
					"pages":     250,
					"publisher": map[string]any{"name": "Test Publishing", "location": "New York"},
				},
			},
			"categories": []any{
				map[string]any{"id": 1, "name": "Technology"},
				map[string]any{"id": 2, "name": "Programming"},
			},
		},
		Markdown: mdocx.MarkdownBundle{
			BundleVersion: mdocx.VersionV1,
			Files:         []mdocx.MarkdownFile{{Path: "readme.md", Content: []byte("# Nested Metadata\n\nThis tests deeply nested metadata structures.\n")}},
		},
		Media: mdocx.MediaBundle{BundleVersion: mdocx.VersionV1},
	}
	return doc, mdocx.CompZSTD, mdocx.CompZSTD
}

func generateMetadataTypes() (*mdocx.Document, mdocx.Compression, mdocx.Compression) {
	doc := &mdocx.Document{
		Metadata: map[string]any{
			"title":         "Metadata Types Test",
			"string_value":  "hello world",
			"int_value":     42,
			"float_value":   3.14159,
			"bool_true":     true,
			"bool_false":    false,
			"null_value":    nil,
			"empty_string":  "",
			"zero_int":      0,
			"negative_int":  -100,
			"large_int":     9007199254740991, // Max safe JS integer
			"scientific":    1.23e10,
			"empty_array":   []any{},
			"empty_object":  map[string]any{},
			"string_array":  []any{"a", "b", "c"},
			"number_array":  []any{1, 2, 3, 4, 5},
			"mixed_array":   []any{1, "two", true, nil, 3.14},
			"nested_arrays": []any{[]any{1, 2}, []any{3, 4}},
		},
		Markdown: mdocx.MarkdownBundle{
			BundleVersion: mdocx.VersionV1,
			Files:         []mdocx.MarkdownFile{{Path: "readme.md", Content: []byte("# Metadata Types\n\nTests various JSON data types in metadata.\n")}},
		},
		Media: mdocx.MediaBundle{BundleVersion: mdocx.VersionV1},
	}
	return doc, mdocx.CompZSTD, mdocx.CompZSTD
}

func generateMetadataUnicode() (*mdocx.Document, mdocx.Compression, mdocx.Compression) {
	doc := &mdocx.Document{
		Metadata: map[string]any{
			"title":       "Unicode Metadata Test 日本語",
			"作者":          "張三",
			"descrição":   "Descrição em português",
			"emoji_key_🎉": "party time",
			"values": map[string]any{
				"japanese":    "こんにちは世界",
				"chinese":     "你好世界",
				"korean":      "안녕하세요",
				"arabic":      "مرحبا بالعالم",
				"hebrew":      "שלום עולם",
				"russian":     "Привет мир",
				"greek":       "Γειά σου Κόσμε",
				"thai":        "สวัสดีโลก",
				"emoji":       "🌍🌎🌏",
				"symbols":     "©®™℃℉",
				"math":        "∑∏∫∂∇",
				"arrows":      "←→↑↓↔↕",
				"box_drawing": "┌─┐│└─┘",
			},
		},
		Markdown: mdocx.MarkdownBundle{
			BundleVersion: mdocx.VersionV1,
			Files:         []mdocx.MarkdownFile{{Path: "readme.md", Content: []byte("# Unicode Metadata\n\nTests unicode in metadata keys and values.\n")}},
		},
		Media: mdocx.MediaBundle{BundleVersion: mdocx.VersionV1},
	}
	return doc, mdocx.CompZSTD, mdocx.CompZSTD
}

// --- Multi-file generators ---

func generateManyFiles() (*mdocx.Document, mdocx.Compression, mdocx.Compression) {
	files := make([]mdocx.MarkdownFile, 50)
	for i := 0; i < 50; i++ {
		files[i] = mdocx.MarkdownFile{
			Path:    fmt.Sprintf("docs/file_%03d.md", i+1),
			Content: []byte(fmt.Sprintf("# File %d\n\nContent for file number %d.\n", i+1, i+1)),
		}
	}
	doc := &mdocx.Document{
		Metadata: map[string]any{
			"title":      "Many Files Test",
			"file_count": 50,
		},
		Markdown: mdocx.MarkdownBundle{
			BundleVersion: mdocx.VersionV1,
			RootPath:      "docs/file_001.md",
			Files:         files,
		},
		Media: mdocx.MediaBundle{BundleVersion: mdocx.VersionV1},
	}
	return doc, mdocx.CompZSTD, mdocx.CompZSTD
}

func generateSiblingDirs() (*mdocx.Document, mdocx.Compression, mdocx.Compression) {
	doc := &mdocx.Document{
		Metadata: map[string]any{"title": "Sibling Directories"},
		Markdown: mdocx.MarkdownBundle{
			BundleVersion: mdocx.VersionV1,
			RootPath:      "index.md",
			Files: []mdocx.MarkdownFile{
				{Path: "index.md", Content: []byte("# Root Index\n")},
				{Path: "docs/api.md", Content: []byte("# API Docs\n")},
				{Path: "docs/guide.md", Content: []byte("# User Guide\n")},
				{Path: "examples/basic.md", Content: []byte("# Basic Example\n")},
				{Path: "examples/advanced.md", Content: []byte("# Advanced Example\n")},
				{Path: "tutorials/intro.md", Content: []byte("# Introduction\n")},
				{Path: "tutorials/intermediate.md", Content: []byte("# Intermediate\n")},
				{Path: "tutorials/advanced.md", Content: []byte("# Advanced Tutorial\n")},
				{Path: "reference/types.md", Content: []byte("# Types Reference\n")},
				{Path: "reference/functions.md", Content: []byte("# Functions Reference\n")},
			},
		},
		Media: mdocx.MediaBundle{BundleVersion: mdocx.VersionV1},
	}
	return doc, mdocx.CompZSTD, mdocx.CompZSTD
}

// --- Media generators ---

func generateMediaNoSHA256() (*mdocx.Document, mdocx.Compression, mdocx.Compression) {
	// SHA256 will be auto-populated by encoder
	doc := &mdocx.Document{
		Metadata: map[string]any{"title": "Media Without SHA256"},
		Markdown: mdocx.MarkdownBundle{
			BundleVersion: mdocx.VersionV1,
			Files: []mdocx.MarkdownFile{
				{Path: "readme.md", Content: []byte("# Auto SHA256\n\nMedia hashes auto-populated.\n"), MediaRefs: []string{"img1", "img2"}},
			},
		},
		Media: mdocx.MediaBundle{
			BundleVersion: mdocx.VersionV1,
			Items: []mdocx.MediaItem{
				{ID: "img1", Path: "a.png", MIMEType: "image/png", Data: []byte{1, 2, 3, 4, 5}},
				{ID: "img2", Path: "b.png", MIMEType: "image/png", Data: []byte{6, 7, 8, 9, 10}},
			},
		},
	}
	return doc, mdocx.CompZSTD, mdocx.CompZSTD
}

func generateMediaMIMETypes() (*mdocx.Document, mdocx.Compression, mdocx.Compression) {
	makeItem := func(id, path, mime string, data []byte) mdocx.MediaItem {
		return mdocx.MediaItem{ID: id, Path: path, MIMEType: mime, Data: data, SHA256: sha256.Sum256(data)}
	}
	doc := &mdocx.Document{
		Metadata: map[string]any{"title": "MIME Types Test"},
		Markdown: mdocx.MarkdownBundle{
			BundleVersion: mdocx.VersionV1,
			Files:         []mdocx.MarkdownFile{{Path: "readme.md", Content: []byte("# MIME Types\n\nVarious media MIME types.\n")}},
		},
		Media: mdocx.MediaBundle{
			BundleVersion: mdocx.VersionV1,
			Items: []mdocx.MediaItem{
				makeItem("png", "img.png", "image/png", []byte{0x89, 'P', 'N', 'G'}),
				makeItem("jpg", "img.jpg", "image/jpeg", []byte{0xFF, 0xD8, 0xFF}),
				makeItem("gif", "img.gif", "image/gif", []byte{'G', 'I', 'F', '8', '9', 'a'}),
				makeItem("webp", "img.webp", "image/webp", []byte{'R', 'I', 'F', 'F'}),
				makeItem("svg", "img.svg", "image/svg+xml", []byte("<svg></svg>")),
				makeItem("mp3", "audio.mp3", "audio/mpeg", []byte{'I', 'D', '3'}),
				makeItem("ogg", "audio.ogg", "audio/ogg", []byte{'O', 'g', 'g', 'S'}),
				makeItem("mp4", "video.mp4", "video/mp4", []byte{0, 0, 0, 0x18, 'f', 't', 'y', 'p'}),
				makeItem("webm", "video.webm", "video/webm", []byte{0x1A, 0x45, 0xDF, 0xA3}),
				makeItem("pdf", "doc.pdf", "application/pdf", []byte{'%', 'P', 'D', 'F'}),
				makeItem("json", "data.json", "application/json", []byte("{}")),
				makeItem("xml", "data.xml", "application/xml", []byte("<?xml?>")),
				makeItem("wasm", "app.wasm", "application/wasm", []byte{0x00, 0x61, 0x73, 0x6D}),
				makeItem("zip", "archive.zip", "application/zip", []byte{'P', 'K', 0x03, 0x04}),
				makeItem("gzip", "archive.gz", "application/gzip", []byte{0x1F, 0x8B}),
				makeItem("bin", "data.bin", "application/octet-stream", []byte{0xDE, 0xAD, 0xBE, 0xEF}),
			},
		},
	}
	return doc, mdocx.CompZSTD, mdocx.CompZSTD
}

func generateMediaBinaryPatterns() (*mdocx.Document, mdocx.Compression, mdocx.Compression) {
	allZeros := make([]byte, 256)
	allOnes := make([]byte, 256)
	for i := range allOnes {
		allOnes[i] = 0xFF
	}
	ascending := make([]byte, 256)
	for i := range ascending {
		ascending[i] = byte(i)
	}
	descending := make([]byte, 256)
	for i := range descending {
		descending[i] = byte(255 - i)
	}
	alternating := make([]byte, 256)
	for i := range alternating {
		if i%2 == 0 {
			alternating[i] = 0xAA
		} else {
			alternating[i] = 0x55
		}
	}

	makeItem := func(id string, data []byte) mdocx.MediaItem {
		return mdocx.MediaItem{ID: id, Path: id + ".bin", MIMEType: "application/octet-stream", Data: data, SHA256: sha256.Sum256(data)}
	}

	doc := &mdocx.Document{
		Metadata: map[string]any{"title": "Binary Patterns Test"},
		Markdown: mdocx.MarkdownBundle{
			BundleVersion: mdocx.VersionV1,
			Files:         []mdocx.MarkdownFile{{Path: "readme.md", Content: []byte("# Binary Patterns\n\nTests specific binary byte patterns.\n")}},
		},
		Media: mdocx.MediaBundle{
			BundleVersion: mdocx.VersionV1,
			Items: []mdocx.MediaItem{
				makeItem("zeros", allZeros),
				makeItem("ones", allOnes),
				makeItem("ascending", ascending),
				makeItem("descending", descending),
				makeItem("alternating", alternating),
			},
		},
	}
	return doc, mdocx.CompZSTD, mdocx.CompZSTD
}

// --- Unicode and special content ---

func generateUnicodePaths() (*mdocx.Document, mdocx.Compression, mdocx.Compression) {
	doc := &mdocx.Document{
		Metadata: map[string]any{"title": "Unicode Paths"},
		Markdown: mdocx.MarkdownBundle{
			BundleVersion: mdocx.VersionV1,
			Files: []mdocx.MarkdownFile{
				{Path: "文档/说明.md", Content: []byte("# 中文文档\n")},
				{Path: "ドキュメント/説明.md", Content: []byte("# 日本語ドキュメント\n")},
				{Path: "문서/설명.md", Content: []byte("# 한국어 문서\n")},
				{Path: "документы/readme.md", Content: []byte("# Русский документ\n")},
				{Path: "émojis/🎉.md", Content: []byte("# Emoji Path\n")},
			},
		},
		Media: mdocx.MediaBundle{BundleVersion: mdocx.VersionV1},
	}
	return doc, mdocx.CompZSTD, mdocx.CompZSTD
}

func generateSpecialMarkdown() (*mdocx.Document, mdocx.Compression, mdocx.Compression) {
	content := []byte(`# Special Markdown Syntax

## Code Blocks

### Inline code
Use ` + "`code`" + ` for inline.

### Fenced code block
` + "```go" + `
package main

import "fmt"

func main() {
    fmt.Println("Hello, World!")
}
` + "```" + `

### Indented code block
    func indented() {
        return true
    }

## Tables

| Column 1 | Column 2 | Column 3 |
|----------|:--------:|---------:|
| Left     | Center   | Right    |
| Data     | Data     | Data     |

## Lists

### Ordered
1. First
2. Second
   1. Nested
   2. Items
3. Third

### Unordered
- Item A
- Item B
  - Nested
  - Items
- Item C

### Task List
- [x] Completed task
- [ ] Incomplete task
- [x] Another done

## Blockquotes

> This is a quote
> 
> > Nested quote
> > More nested

## Horizontal Rules

---
***
___

## Links and Images

[Regular link](https://example.com)
[Reference link][ref]
![Alt text](image.png "Title")

[ref]: https://example.com

## HTML

<details>
<summary>Click to expand</summary>
Hidden content here.
</details>

## Escapes

\*not italic\*
\` + "`" + `not code\` + "`" + `
\[not a link\]
`)

	doc := &mdocx.Document{
		Metadata: map[string]any{"title": "Special Markdown"},
		Markdown: mdocx.MarkdownBundle{
			BundleVersion: mdocx.VersionV1,
			Files:         []mdocx.MarkdownFile{{Path: "special.md", Content: content}},
		},
		Media: mdocx.MediaBundle{BundleVersion: mdocx.VersionV1},
	}
	return doc, mdocx.CompZSTD, mdocx.CompZSTD
}

// --- Edge case generators ---

func generateEmptyContent() (*mdocx.Document, mdocx.Compression, mdocx.Compression) {
	doc := &mdocx.Document{
		Metadata: map[string]any{"title": "Empty Content"},
		Markdown: mdocx.MarkdownBundle{
			BundleVersion: mdocx.VersionV1,
			Files: []mdocx.MarkdownFile{
				{Path: "empty.md", Content: []byte{}},
				{Path: "nonempty.md", Content: []byte("# Not Empty\n")},
			},
		},
		Media: mdocx.MediaBundle{BundleVersion: mdocx.VersionV1},
	}
	return doc, mdocx.CompZSTD, mdocx.CompZSTD
}

func generateWhitespaceContent() (*mdocx.Document, mdocx.Compression, mdocx.Compression) {
	doc := &mdocx.Document{
		Metadata: map[string]any{"title": "Whitespace Content"},
		Markdown: mdocx.MarkdownBundle{
			BundleVersion: mdocx.VersionV1,
			Files: []mdocx.MarkdownFile{
				{Path: "spaces.md", Content: []byte("   ")},
				{Path: "tabs.md", Content: []byte("\t\t\t")},
				{Path: "newlines.md", Content: []byte("\n\n\n")},
				{Path: "mixed.md", Content: []byte("  \t\n  \t\n")},
				{Path: "crlf.md", Content: []byte("\r\n\r\n")},
				{Path: "trailing.md", Content: []byte("# Title\n\n\n\n")},
			},
		},
		Media: mdocx.MediaBundle{BundleVersion: mdocx.VersionV1},
	}
	return doc, mdocx.CompZSTD, mdocx.CompZSTD
}

func generateSingleChar() (*mdocx.Document, mdocx.Compression, mdocx.Compression) {
	doc := &mdocx.Document{
		Metadata: map[string]any{"title": "Single Character"},
		Markdown: mdocx.MarkdownBundle{
			BundleVersion: mdocx.VersionV1,
			Files: []mdocx.MarkdownFile{
				{Path: "a.md", Content: []byte("a")},
				{Path: "newline.md", Content: []byte("\n")},
				{Path: "hash.md", Content: []byte("#")},
				{Path: "emoji.md", Content: []byte("🎉")},
			},
		},
		Media: mdocx.MediaBundle{BundleVersion: mdocx.VersionV1},
	}
	return doc, mdocx.CompZSTD, mdocx.CompZSTD
}

// --- Attribute generators ---

func generateAttributesEmpty() (*mdocx.Document, mdocx.Compression, mdocx.Compression) {
	doc := &mdocx.Document{
		Metadata: map[string]any{"title": "Empty Attributes"},
		Markdown: mdocx.MarkdownBundle{
			BundleVersion: mdocx.VersionV1,
			Files: []mdocx.MarkdownFile{
				{Path: "with_empty.md", Content: []byte("# Has Empty Attributes\n"), Attributes: map[string]string{}},
				{Path: "with_nil.md", Content: []byte("# Has Nil Attributes\n"), Attributes: nil},
				{Path: "with_values.md", Content: []byte("# Has Values\n"), Attributes: map[string]string{"key": "value"}},
			},
		},
		Media: mdocx.MediaBundle{BundleVersion: mdocx.VersionV1},
	}
	return doc, mdocx.CompZSTD, mdocx.CompZSTD
}

func generateAttributesUnicode() (*mdocx.Document, mdocx.Compression, mdocx.Compression) {
	data := []byte{1, 2, 3}
	hash := sha256.Sum256(data)
	doc := &mdocx.Document{
		Metadata: map[string]any{"title": "Unicode Attributes"},
		Markdown: mdocx.MarkdownBundle{
			BundleVersion: mdocx.VersionV1,
			Files: []mdocx.MarkdownFile{
				{
					Path:    "unicode_attrs.md",
					Content: []byte("# Unicode Attributes\n"),
					Attributes: map[string]string{
						"日本語キー":   "日本語の値",
						"emoji_🎉": "party value",
						"中文":      "测试值",
						"한국어":     "테스트",
					},
				},
			},
		},
		Media: mdocx.MediaBundle{
			BundleVersion: mdocx.VersionV1,
			Items: []mdocx.MediaItem{
				{
					ID: "media1", Path: "data.bin", MIMEType: "application/octet-stream",
					Data: data, SHA256: hash,
					Attributes: map[string]string{"描述": "二进制数据", "설명": "바이너리 데이터"},
				},
			},
		},
	}
	return doc, mdocx.CompZSTD, mdocx.CompZSTD
}

// --- Path variation generators ---

func generateDotfilePaths() (*mdocx.Document, mdocx.Compression, mdocx.Compression) {
	doc := &mdocx.Document{
		Metadata: map[string]any{"title": "Dotfile Paths"},
		Markdown: mdocx.MarkdownBundle{
			BundleVersion: mdocx.VersionV1,
			Files: []mdocx.MarkdownFile{
				{Path: ".hidden.md", Content: []byte("# Hidden File\n")},
				{Path: ".config/settings.md", Content: []byte("# Config Settings\n")},
				{Path: "docs/.gitkeep.md", Content: []byte("# Gitkeep\n")},
				{Path: "normal.md", Content: []byte("# Normal File\n")},
			},
		},
		Media: mdocx.MediaBundle{BundleVersion: mdocx.VersionV1},
	}
	return doc, mdocx.CompZSTD, mdocx.CompZSTD
}

func generateExtensionVariety() (*mdocx.Document, mdocx.Compression, mdocx.Compression) {
	doc := &mdocx.Document{
		Metadata: map[string]any{"title": "Extension Variety"},
		Markdown: mdocx.MarkdownBundle{
			BundleVersion: mdocx.VersionV1,
			Files: []mdocx.MarkdownFile{
				{Path: "file.md", Content: []byte("# Standard .md\n")},
				{Path: "file.markdown", Content: []byte("# Standard .markdown\n")},
				{Path: "file.txt", Content: []byte("# Plain text\n")},
				{Path: "file.mdx", Content: []byte("# MDX format\n")},
				{Path: "file.MD", Content: []byte("# Uppercase .MD\n")},
				{Path: "file.MARKDOWN", Content: []byte("# Uppercase .MARKDOWN\n")},
				{Path: "no_extension", Content: []byte("# No extension\n")},
				{Path: "dots.in.name.md", Content: []byte("# Multiple dots\n")},
			},
		},
		Media: mdocx.MediaBundle{BundleVersion: mdocx.VersionV1},
	}
	return doc, mdocx.CompZSTD, mdocx.CompZSTD
}

// --- Large content generators ---

func generateLargeSingleFile() (*mdocx.Document, mdocx.Compression, mdocx.Compression) {
	var content []byte
	content = append(content, []byte("# Large Single File Test\n\n")...)
	content = append(content, []byte("This file is designed to be over 100KB to test large file handling.\n\n")...)

	// Generate ~120KB of content
	for i := 0; i < 500; i++ {
		content = append(content, []byte(fmt.Sprintf("## Section %d\n\n", i+1))...)
		content = append(content, []byte("Lorem ipsum dolor sit amet, consectetur adipiscing elit. Sed do eiusmod tempor incididunt ut labore et dolore magna aliqua. Ut enim ad minim veniam, quis nostrud exercitation ullamco laboris nisi ut aliquip ex ea commodo consequat. Duis aute irure dolor in reprehenderit in voluptate velit esse cillum dolore eu fugiat nulla pariatur.\n\n")...)
	}

	doc := &mdocx.Document{
		Metadata: map[string]any{"title": "Large Single File", "size_kb": len(content) / 1024},
		Markdown: mdocx.MarkdownBundle{
			BundleVersion: mdocx.VersionV1,
			Files:         []mdocx.MarkdownFile{{Path: "large.md", Content: content}},
		},
		Media: mdocx.MediaBundle{BundleVersion: mdocx.VersionV1},
	}
	return doc, mdocx.CompZSTD, mdocx.CompZSTD
}

func generateLargeMedia() (*mdocx.Document, mdocx.Compression, mdocx.Compression) {
	// Generate 10KB media items
	makeData := func(seed byte, size int) []byte {
		data := make([]byte, size)
		for i := range data {
			data[i] = byte((int(seed) + i) % 256)
		}
		return data
	}

	items := make([]mdocx.MediaItem, 5)
	for i := 0; i < 5; i++ {
		data := makeData(byte(i*50), 10240) // 10KB each
		items[i] = mdocx.MediaItem{
			ID:       fmt.Sprintf("media_%d", i+1),
			Path:     fmt.Sprintf("large_%d.bin", i+1),
			MIMEType: "application/octet-stream",
			Data:     data,
			SHA256:   sha256.Sum256(data),
		}
	}

	doc := &mdocx.Document{
		Metadata: map[string]any{"title": "Large Media Items", "total_media_kb": 50},
		Markdown: mdocx.MarkdownBundle{
			BundleVersion: mdocx.VersionV1,
			Files:         []mdocx.MarkdownFile{{Path: "readme.md", Content: []byte("# Large Media\n\n5 media items at 10KB each.\n")}},
		},
		Media: mdocx.MediaBundle{BundleVersion: mdocx.VersionV1, Items: items},
	}
	return doc, mdocx.CompZSTD, mdocx.CompZSTD
}

// --- Realistic example generators ---

func generateBlogPost() (*mdocx.Document, mdocx.Compression, mdocx.Compression) {
	content := []byte(`# How to Build a REST API in Go

*Published on January 7, 2026 by Jane Developer*

![Header Image](mdocx://media/header)

## Introduction

Building REST APIs in Go is straightforward thanks to the standard library's excellent HTTP support. In this tutorial, we'll build a complete CRUD API for a todo application.

## Prerequisites

- Go 1.21 or later
- Basic understanding of HTTP concepts
- A text editor or IDE

## Project Setup

First, create a new directory and initialize a Go module:

` + "```bash" + `
mkdir todo-api
cd todo-api
go mod init github.com/example/todo-api
` + "```" + `

## Creating the Server

Let's start with a basic HTTP server:

` + "```go" + `
package main

import (
    "log"
    "net/http"
)

func main() {
    mux := http.NewServeMux()
    mux.HandleFunc("/", handleRoot)
    
    log.Println("Server starting on :8080")
    log.Fatal(http.ListenAndServe(":8080", mux))
}

func handleRoot(w http.ResponseWriter, r *http.Request) {
    w.Write([]byte("Hello, World!"))
}
` + "```" + `

## Conclusion

We've built a complete REST API! For the full source code, check out the [GitHub repository](https://github.com/example/todo-api).

---

*Tags: go, golang, rest, api, tutorial*
`)

	headerImg := []byte{0x89, 'P', 'N', 'G', 0x0D, 0x0A, 0x1A, 0x0A, 0, 0, 0, 13, 'I', 'H', 'D', 'R'}

	doc := &mdocx.Document{
		Metadata: map[string]any{
			"title":        "How to Build a REST API in Go",
			"author":       "Jane Developer",
			"published_at": "2026-01-07T10:00:00Z",
			"tags":         []any{"go", "golang", "rest", "api", "tutorial"},
			"category":     "tutorials",
			"reading_time": "8 min",
		},
		Markdown: mdocx.MarkdownBundle{
			BundleVersion: mdocx.VersionV1,
			Files: []mdocx.MarkdownFile{
				{Path: "post.md", Content: content, MediaRefs: []string{"header"}, Attributes: map[string]string{"status": "published"}},
			},
		},
		Media: mdocx.MediaBundle{
			BundleVersion: mdocx.VersionV1,
			Items: []mdocx.MediaItem{
				{ID: "header", Path: "images/header.png", MIMEType: "image/png", Data: headerImg, SHA256: sha256.Sum256(headerImg)},
			},
		},
	}
	return doc, mdocx.CompZSTD, mdocx.CompZSTD
}

func generateAPIDocs() (*mdocx.Document, mdocx.Compression, mdocx.Compression) {
	indexContent := []byte(`# API Documentation

Welcome to the API documentation. This API provides access to our platform's resources.

## Endpoints

- [Authentication](auth.md)
- [Users](users.md)
- [Resources](resources.md)

## Base URL

All API requests should be made to:

` + "```" + `
https://api.example.com/v1
` + "```" + `

## Authentication

All endpoints require authentication via Bearer token. See [Authentication](auth.md) for details.
`)

	authContent := []byte(`# Authentication

## Overview

This API uses JWT tokens for authentication.

## Obtaining a Token

` + "```http" + `
POST /auth/token
Content-Type: application/json

{
    "username": "user@example.com",
    "password": "secret"
}
` + "```" + `

### Response

` + "```json" + `
{
    "token": "eyJhbGciOiJIUzI1NiIs...",
    "expires_in": 3600,
    "token_type": "Bearer"
}
` + "```" + `

## Using the Token

Include the token in the Authorization header:

` + "```http" + `
GET /users/me
Authorization: Bearer eyJhbGciOiJIUzI1NiIs...
` + "```" + `
`)

	usersContent := []byte(`# Users API

## List Users

` + "```http" + `
GET /users
` + "```" + `

### Query Parameters

| Parameter | Type    | Description           |
|-----------|---------|----------------------|
| page      | integer | Page number (default: 1) |
| limit     | integer | Items per page (default: 20) |
| search    | string  | Search by name or email |

### Response

` + "```json" + `
{
    "data": [
        {
            "id": "123",
            "name": "John Doe",
            "email": "john@example.com"
        }
    ],
    "meta": {
        "total": 100,
        "page": 1,
        "limit": 20
    }
}
` + "```" + `

## Get User

` + "```http" + `
GET /users/{id}
` + "```" + `

## Create User

` + "```http" + `
POST /users
Content-Type: application/json

{
    "name": "Jane Doe",
    "email": "jane@example.com"
}
` + "```" + `
`)

	doc := &mdocx.Document{
		Metadata: map[string]any{
			"title":       "API Documentation",
			"version":     "1.0.0",
			"description": "Complete API reference documentation",
			"root":        "index.md",
		},
		Markdown: mdocx.MarkdownBundle{
			BundleVersion: mdocx.VersionV1,
			RootPath:      "index.md",
			Files: []mdocx.MarkdownFile{
				{Path: "index.md", Content: indexContent, Attributes: map[string]string{"type": "index"}},
				{Path: "auth.md", Content: authContent, Attributes: map[string]string{"type": "endpoint"}},
				{Path: "users.md", Content: usersContent, Attributes: map[string]string{"type": "endpoint"}},
				{Path: "resources.md", Content: []byte("# Resources API\n\nComing soon.\n"), Attributes: map[string]string{"type": "endpoint", "status": "draft"}},
			},
		},
		Media: mdocx.MediaBundle{BundleVersion: mdocx.VersionV1},
	}
	return doc, mdocx.CompZSTD, mdocx.CompZSTD
}

func generateBookChapter() (*mdocx.Document, mdocx.Compression, mdocx.Compression) {
	tocContent := []byte(`# The Go Programming Language

## Table of Contents

1. [Introduction](ch01-intro.md)
2. [Getting Started](ch02-getting-started.md)
3. [Basic Types](ch03-types.md)
4. [Functions](ch04-functions.md)
5. [Packages](ch05-packages.md)

---

*© 2026 Example Publishing*
`)

	ch1Content := []byte(`# Chapter 1: Introduction

Go is an open source programming language that makes it easy to build simple, reliable, and efficient software.

## History

Go was designed at Google in 2007 by Robert Griesemer, Rob Pike, and Ken Thompson. It was announced in November 2009 and version 1.0 was released in March 2012.

## Key Features

- **Simplicity**: Go has a clean, simple syntax
- **Concurrency**: Built-in support for concurrent programming
- **Fast compilation**: Near-instant compile times
- **Garbage collection**: Automatic memory management

## Who Uses Go?

Many companies use Go in production:

- Google
- Uber
- Dropbox
- Cloudflare
- Docker

> "Go is expressive, concise, clean, and efficient."
> — The Go Authors

[Next: Getting Started →](ch02-getting-started.md)
`)

	ch2Content := []byte(`# Chapter 2: Getting Started

In this chapter, we'll install Go and write our first program.

## Installation

### macOS

` + "```bash" + `
brew install go
` + "```" + `

### Linux

` + "```bash" + `
wget https://go.dev/dl/go1.21.linux-amd64.tar.gz
sudo tar -C /usr/local -xzf go1.21.linux-amd64.tar.gz
` + "```" + `

## Hello, World!

Create a file named ` + "`hello.go`" + `:

` + "```go" + `
package main

import "fmt"

func main() {
    fmt.Println("Hello, World!")
}
` + "```" + `

Run it:

` + "```bash" + `
go run hello.go
` + "```" + `

[← Previous: Introduction](ch01-intro.md) | [Next: Basic Types →](ch03-types.md)
`)

	doc := &mdocx.Document{
		Metadata: map[string]any{
			"title":   "The Go Programming Language",
			"author":  "Example Author",
			"isbn":    "978-0-123456-78-9",
			"edition": 1,
			"year":    2026,
			"root":    "toc.md",
		},
		Markdown: mdocx.MarkdownBundle{
			BundleVersion: mdocx.VersionV1,
			RootPath:      "toc.md",
			Files: []mdocx.MarkdownFile{
				{Path: "toc.md", Content: tocContent, Attributes: map[string]string{"type": "toc"}},
				{Path: "ch01-intro.md", Content: ch1Content, Attributes: map[string]string{"chapter": "1", "title": "Introduction"}},
				{Path: "ch02-getting-started.md", Content: ch2Content, Attributes: map[string]string{"chapter": "2", "title": "Getting Started"}},
				{Path: "ch03-types.md", Content: []byte("# Chapter 3: Basic Types\n\n*Coming soon*\n"), Attributes: map[string]string{"chapter": "3", "status": "draft"}},
				{Path: "ch04-functions.md", Content: []byte("# Chapter 4: Functions\n\n*Coming soon*\n"), Attributes: map[string]string{"chapter": "4", "status": "draft"}},
				{Path: "ch05-packages.md", Content: []byte("# Chapter 5: Packages\n\n*Coming soon*\n"), Attributes: map[string]string{"chapter": "5", "status": "draft"}},
			},
		},
		Media: mdocx.MediaBundle{BundleVersion: mdocx.VersionV1},
	}
	return doc, mdocx.CompZSTD, mdocx.CompZSTD
}

func generateReadmeProject() (*mdocx.Document, mdocx.Compression, mdocx.Compression) {
	content := []byte(`# go-awesome-project

[![Go Version](https://img.shields.io/badge/go-1.21+-blue.svg)](https://go.dev/)
[![License](https://img.shields.io/badge/license-MIT-green.svg)](LICENSE)
[![Build Status](https://github.com/example/go-awesome-project/workflows/CI/badge.svg)](https://github.com/example/go-awesome-project/actions)
[![Coverage](https://codecov.io/gh/example/go-awesome-project/branch/main/graph/badge.svg)](https://codecov.io/gh/example/go-awesome-project)
[![Go Report Card](https://goreportcard.com/badge/github.com/example/go-awesome-project)](https://goreportcard.com/report/github.com/example/go-awesome-project)

A blazingly fast, highly configurable, and incredibly awesome Go library for doing amazing things.

![Demo](mdocx://media/demo)

## Features

- 🚀 **Fast**: Optimized for performance
- 🔒 **Secure**: Built with security in mind
- 📦 **Zero dependencies**: No external dependencies
- 🧪 **Well tested**: 95%+ test coverage
- 📚 **Well documented**: Comprehensive documentation

## Installation

` + "```bash" + `
go get github.com/example/go-awesome-project
` + "```" + `

## Quick Start

` + "```go" + `
package main

import (
    "fmt"
    awesome "github.com/example/go-awesome-project"
)

func main() {
    result := awesome.DoSomething("input")
    fmt.Println(result)
}
` + "```" + `

## Documentation

For detailed documentation, see [docs/](docs/).

## Contributing

Contributions are welcome! Please read our [Contributing Guide](CONTRIBUTING.md) before submitting a PR.

## License

MIT License - see [LICENSE](LICENSE) for details.
`)

	demoGif := []byte{'G', 'I', 'F', '8', '9', 'a', 0x01, 0x00, 0x01, 0x00}

	doc := &mdocx.Document{
		Metadata: map[string]any{
			"title":       "go-awesome-project",
			"description": "A blazingly fast Go library for doing amazing things",
			"license":     "MIT",
			"repository":  "https://github.com/example/go-awesome-project",
			"go_version":  "1.21+",
		},
		Markdown: mdocx.MarkdownBundle{
			BundleVersion: mdocx.VersionV1,
			Files: []mdocx.MarkdownFile{
				{Path: "README.md", Content: content, MediaRefs: []string{"demo"}},
			},
		},
		Media: mdocx.MediaBundle{
			BundleVersion: mdocx.VersionV1,
			Items: []mdocx.MediaItem{
				{ID: "demo", Path: "assets/demo.gif", MIMEType: "image/gif", Data: demoGif, SHA256: sha256.Sum256(demoGif)},
			},
		},
	}
	return doc, mdocx.CompZSTD, mdocx.CompZSTD
}

// --- Compression stress test generators ---

func generateHighlyCompressible() (*mdocx.Document, mdocx.Compression, mdocx.Compression) {
	// Highly repetitive content
	var content []byte
	content = append(content, []byte("# Highly Compressible Content\n\n")...)
	line := []byte("This is a repeated line of text that should compress extremely well. ")
	for i := 0; i < 1000; i++ {
		content = append(content, line...)
	}

	// Highly repetitive binary data
	binaryData := make([]byte, 10240)
	for i := range binaryData {
		binaryData[i] = byte(i % 4) // Only 4 unique values, very compressible
	}

	doc := &mdocx.Document{
		Metadata: map[string]any{"title": "Highly Compressible", "pattern": "repetitive"},
		Markdown: mdocx.MarkdownBundle{
			BundleVersion: mdocx.VersionV1,
			Files:         []mdocx.MarkdownFile{{Path: "compressible.md", Content: content}},
		},
		Media: mdocx.MediaBundle{
			BundleVersion: mdocx.VersionV1,
			Items: []mdocx.MediaItem{
				{ID: "data", Path: "data.bin", MIMEType: "application/octet-stream", Data: binaryData, SHA256: sha256.Sum256(binaryData)},
			},
		},
	}
	return doc, mdocx.CompZSTD, mdocx.CompZSTD
}

func generateIncompressible() (*mdocx.Document, mdocx.Compression, mdocx.Compression) {
	// Pseudo-random content that doesn't compress well
	// Using a simple LCG for deterministic "random" data
	lcg := uint32(12345)
	nextRand := func() byte {
		lcg = lcg*1103515245 + 12345
		return byte((lcg >> 16) & 0xFF)
	}

	content := make([]byte, 4096)
	for i := range content {
		content[i] = nextRand()
		// Keep it valid UTF-8 by limiting to printable ASCII
		content[i] = 32 + (content[i] % 95)
	}

	binaryData := make([]byte, 4096)
	for i := range binaryData {
		binaryData[i] = nextRand()
	}

	doc := &mdocx.Document{
		Metadata: map[string]any{"title": "Incompressible Content", "pattern": "random"},
		Markdown: mdocx.MarkdownBundle{
			BundleVersion: mdocx.VersionV1,
			Files:         []mdocx.MarkdownFile{{Path: "random.md", Content: content}},
		},
		Media: mdocx.MediaBundle{
			BundleVersion: mdocx.VersionV1,
			Items: []mdocx.MediaItem{
				{ID: "random", Path: "random.bin", MIMEType: "application/octet-stream", Data: binaryData, SHA256: sha256.Sum256(binaryData)},
			},
		},
	}
	return doc, mdocx.CompZSTD, mdocx.CompZSTD
}
