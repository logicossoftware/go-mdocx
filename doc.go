// Package mdocx implements the MDOCX (MarkDown Open Container eXchange) container format.
//
// MDOCX is a single-file container format for bundling one or more Markdown documents
// together with referenced binary media (images, audio, video, etc.). It is designed
// for exchange, archival, and transport of rich Markdown content.
//
// # File Format Overview
//
// An MDOCX file consists of:
//   - A 32-byte fixed header with magic bytes, version, and flags
//   - An optional UTF-8 JSON metadata block
//   - A Markdown bundle section containing one or more Markdown files
//   - A Media bundle section containing zero or more media items
//
// Payloads are serialized using Go's encoding/gob and optionally compressed using
// ZIP, Zstandard, LZ4, or Brotli compression.
//
// # Basic Usage
//
// To create and write an MDOCX file:
//
//	doc := &mdocx.Document{
//		Metadata: map[string]any{"title": "My Document"},
//		Markdown: mdocx.MarkdownBundle{
//			BundleVersion: mdocx.VersionV1,
//			Files: []mdocx.MarkdownFile{
//				{Path: "readme.md", Content: []byte("# Hello World")},
//			},
//		},
//		Media: mdocx.MediaBundle{BundleVersion: mdocx.VersionV1},
//	}
//	f, _ := os.Create("output.mdocx")
//	defer f.Close()
//	err := mdocx.Encode(f, doc)
//
// To read an MDOCX file:
//
//	f, _ := os.Open("input.mdocx")
//	defer f.Close()
//	doc, err := mdocx.Decode(f)
//
// # Security Considerations
//
// The package includes built-in protection against oversized allocations and
// decompression bombs via configurable [Limits]. All size limits are enforced
// during decoding to prevent resource exhaustion attacks.
//
// # Specification
//
// The complete format specification is defined in rfc.md at the repository root.
package mdocx
