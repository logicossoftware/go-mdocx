# go-mdocx

[![Go Reference](https://pkg.go.dev/badge/github.com/logicossoftware/go-mdocx.svg)](https://pkg.go.dev/github.com/logicossoftware/go-mdocx)
[![Go Report Card](https://goreportcard.com/badge/github.com/logicossoftware/go-mdocx)](https://goreportcard.com/report/github.com/logicossoftware/go-mdocx)
[![License: MIT](https://img.shields.io/badge/License-MIT-blue.svg)](LICENSE)

Go implementation of the **MDOCX** (MarkDown Open Container eXchange) file format — a single-file container for bundling Markdown documents with referenced media (images, audio, video, etc.).

## Features

- 📦 **Single-file container** — Bundle multiple Markdown files and media assets into one `.mdocx` file
- 🗜️ **Multiple compression options** — Zstandard (default), ZIP, LZ4, Brotli, or no compression
- 🔒 **SHA-256 integrity** — Optional hash verification for media items
- 📝 **Rich metadata** — JSON metadata block for document properties
- 🔗 **Media reference resolution** — Resolve `mdocx://media/<ID>` URIs and relative paths
- ⚡ **Efficient parsing** — Deterministic header and length-delimited sections
- 🛡️ **Security built-in** — Configurable limits protect against resource exhaustion

## Installation

```bash
go get github.com/logicossoftware/go-mdocx
```

Requires Go 1.21 or later.

## Quick Start

### Creating an MDOCX file

```go
package main

import (
    "os"
    "github.com/logicossoftware/go-mdocx"
)

func main() {
    doc := &mdocx.Document{
        Metadata: map[string]any{
            "title":   "My Document",
            "creator": "Jane Doe",
        },
        Markdown: mdocx.MarkdownBundle{
            BundleVersion: mdocx.VersionV1,
            Files: []mdocx.MarkdownFile{
                {
                    Path:    "readme.md",
                    Content: []byte("# Hello World\n\nThis is my document."),
                },
            },
        },
        Media: mdocx.MediaBundle{BundleVersion: mdocx.VersionV1},
    }

    f, _ := os.Create("output.mdocx")
    defer f.Close()
    
    err := mdocx.Encode(f, doc)
    if err != nil {
        panic(err)
    }
}
```

### Reading an MDOCX file

```go
package main

import (
    "fmt"
    "os"
    "github.com/logicossoftware/go-mdocx"
)

func main() {
    f, _ := os.Open("input.mdocx")
    defer f.Close()

    doc, err := mdocx.Decode(f)
    if err != nil {
        panic(err)
    }

    fmt.Printf("Title: %v\n", doc.Metadata["title"])
    fmt.Printf("Files: %d\n", len(doc.Markdown.Files))
    fmt.Printf("Media: %d\n", len(doc.Media.Items))
}
```

### With media and compression options

```go
import "crypto/sha256"

// Create document with media
imageData := []byte{...} // your image bytes
hash := sha256.Sum256(imageData)

doc := &mdocx.Document{
    Metadata: map[string]any{"title": "Document with Image"},
    Markdown: mdocx.MarkdownBundle{
        BundleVersion: mdocx.VersionV1,
        Files: []mdocx.MarkdownFile{
            {
                Path:      "readme.md",
                Content:   []byte("# My Doc\n\n![Logo](mdocx://media/logo)"),
                MediaRefs: []string{"logo"},
            },
        },
    },
    Media: mdocx.MediaBundle{
        BundleVersion: mdocx.VersionV1,
        Items: []mdocx.MediaItem{
            {
                ID:       "logo",
                Path:     "assets/logo.png",
                MIMEType: "image/png",
                Data:     imageData,
                SHA256:   hash,
            },
        },
    },
}

// Encode with custom compression
err := mdocx.Encode(f, doc,
    mdocx.WithMarkdownCompression(mdocx.CompZSTD),
    mdocx.WithMediaCompression(mdocx.CompLZ4),
)
```

## Compression Options

| Constant | Algorithm | Best For |
|----------|-----------|----------|
| `CompZSTD` | Zstandard | Default, balanced speed/ratio |
| `CompZIP` | ZIP (DEFLATE) | Maximum interoperability |
| `CompLZ4` | LZ4 | Maximum speed |
| `CompBR` | Brotli | Maximum compression ratio |
| `CompNone` | None | No compression |

## Encoding Options

```go
mdocx.Encode(w, doc,
    mdocx.WithMarkdownCompression(mdocx.CompZSTD),  // Compression for markdown section
    mdocx.WithMediaCompression(mdocx.CompZSTD),     // Compression for media section
    mdocx.WithAutoPopulateSHA256(true),             // Auto-compute SHA256 for media (default: true)
    mdocx.WithVerifyHashesOnWrite(true),            // Verify hashes before writing (default: true)
    mdocx.WithWriteLimits(mdocx.DefaultLimits()),   // Custom size limits
)
```

## Decoding Options

```go
doc, err := mdocx.Decode(r,
    mdocx.WithVerifyHashes(true),                   // Verify SHA256 on media items (default: true)
    mdocx.WithReadLimits(mdocx.Limits{              // Custom size limits
        MaxMediaUncompressed: 4 << 30,              // Allow up to 4 GiB media
    }),
)
```

## Default Limits

The package enforces configurable limits to protect against malicious input:

| Limit | Default Value |
|-------|---------------|
| MaxMetadataLen | 1 MiB |
| MaxMarkdownUncompressed | 256 MiB |
| MaxMediaUncompressed | 2 GiB |
| MaxMarkdownFiles | 10,000 |
| MaxMediaItems | 10,000 |
| MaxSingleMarkdownFileSize | 256 MiB |
| MaxSingleMediaSize | 512 MiB |

## Examples

The [`examples/`](examples/) directory contains runnable examples:

| Example | Description |
|---------|-------------|
| [`create-basic`](examples/create-basic/) | Create a minimal `.mdocx` file |
| [`inspect`](examples/inspect/) | Print a summary of an `.mdocx` file |
| [`pack-dir`](examples/pack-dir/) | Pack a directory of markdown + assets |
| [`unpack`](examples/unpack/) | Extract an `.mdocx` back to disk |
| [`validate`](examples/validate/) | Validate and output JSON (cross-language testing) |

Run an example:

```bash
cd examples/create-basic
go run .
```

## File Format

MDOCX files consist of:

1. **Fixed header** (32 bytes) — Magic bytes, version, flags
2. **Metadata block** (optional) — UTF-8 JSON
3. **Markdown section** — One or more Markdown files (gob-encoded, optionally compressed)
4. **Media section** — Zero or more media items (gob-encoded, optionally compressed)

For the complete specification, see [`rfc.md`](rfc.md).

### Magic Bytes

```
4D 44 4F 43 58 0D 0A 1A  ("MDOCX\r\n" + 0x1A)
```

## Media References

Reference media from Markdown using:

- **By ID**: `![Logo](mdocx://media/logo_id)`
- **By path**: `![Logo](assets/logo.png)` (if `MediaItem.Path` is set)

## Media Resolution

The package provides a `MediaResolver` for resolving media references:

```go
// Create resolver from document
resolver := mdocx.NewMediaResolver(doc)

// Get by ID
item := resolver.GetByID("logo")

// Get by path
item := resolver.GetByPath("assets/logo.png")

// Resolve any reference (mdocx:// URI or path)
item := resolver.Resolve("mdocx://media/logo")
item := resolver.Resolve("assets/logo.png")

// Check existence
if resolver.HasID("logo") {
    // ...
}

// Get all media referenced by a markdown file
refs := resolver.GetReferencedMedia(&doc.Markdown.Files[0])

// List all IDs and paths
ids := resolver.IDs()
paths := resolver.Paths()
```

### List Media Contents

Read only media items without fully decoding the markdown bundle:

```go
f, _ := os.Open("document.mdocx")
defer f.Close()

items, err := mdocx.ListMediaContents(f)
for _, item := range items {
    fmt.Printf("%s: %s (%d bytes)\n", item.ID, item.MIMEType, len(item.Data))
}
```

### Parse Media Reference

Parse a reference string to determine its type:

```go
ref := mdocx.ParseMediaReference("mdocx://media/logo")
// ref.Type == "id", ref.ID == "logo"

ref := mdocx.ParseMediaReference("assets/image.png")
// ref.Type == "path", ref.Path == "assets/image.png"
```

## API Reference

See the [Go package documentation](https://pkg.go.dev/github.com/logicossoftware/go-mdocx) for complete API details.

### Core Types

- [`Document`](https://pkg.go.dev/github.com/logicossoftware/go-mdocx#Document) — Top-level container
- [`MarkdownBundle`](https://pkg.go.dev/github.com/logicossoftware/go-mdocx#MarkdownBundle) — Collection of Markdown files
- [`MarkdownFile`](https://pkg.go.dev/github.com/logicossoftware/go-mdocx#MarkdownFile) — Single Markdown document
- [`MediaBundle`](https://pkg.go.dev/github.com/logicossoftware/go-mdocx#MediaBundle) — Collection of media items
- [`MediaItem`](https://pkg.go.dev/github.com/logicossoftware/go-mdocx#MediaItem) — Single media asset

### Core Functions

- [`Encode`](https://pkg.go.dev/github.com/logicossoftware/go-mdocx#Encode) — Write a document to an MDOCX file
- [`Decode`](https://pkg.go.dev/github.com/logicossoftware/go-mdocx#Decode) — Read a document from an MDOCX file
- [`ListMediaContents`](https://pkg.go.dev/github.com/logicossoftware/go-mdocx#ListMediaContents) — List media items without full decode

### Media Resolution

- [`MediaResolver`](https://pkg.go.dev/github.com/logicossoftware/go-mdocx#MediaResolver) — Resolve media by ID, path, or URI
- [`NewMediaResolver`](https://pkg.go.dev/github.com/logicossoftware/go-mdocx#NewMediaResolver) — Create a new resolver
- [`ParseMediaReference`](https://pkg.go.dev/github.com/logicossoftware/go-mdocx#ParseMediaReference) — Parse a reference string

## Contributing

Contributions are welcome! Please feel free to submit issues and pull requests.

## License

[MIT License](LICENSE) — Copyright (c) 2026, MHJ Wiggers / Logicos Software
