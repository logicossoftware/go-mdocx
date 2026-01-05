package main

import (
	"flag"
	"fmt"
	"log"
	"os"
	"time"

	"github.com/logicossoftware/go-mdocx"
)

func main() {
	var outPath string
	var title string
	var mdPath string
	var mdText string
	var mediaFSPath string
	var mediaID string
	var mediaMIME string
	var mediaContainerPath string

	flag.StringVar(&outPath, "out", "sample.mdocx", "output .mdocx file")
	flag.StringVar(&title, "title", "Example", "document title")
	flag.StringVar(&mdPath, "md", "docs/index.md", "markdown container path")
	flag.StringVar(&mdText, "md-text", "# Hello\n\nThis is MDOCX.\n", "markdown content")
	flag.StringVar(&mediaFSPath, "media", "", "optional media file path on disk")
	flag.StringVar(&mediaID, "media-id", "media_1", "media ID")
	flag.StringVar(&mediaMIME, "media-mime", "application/octet-stream", "media MIME type")
	flag.StringVar(&mediaContainerPath, "media-path", "assets/media.bin", "media container path")
	flag.Parse()

	doc := &mdocx.Document{
		Metadata: map[string]any{
			"title":      title,
			"created_at": time.Now().UTC().Format(time.RFC3339),
			"root":       mdPath,
		},
		Markdown: mdocx.MarkdownBundle{
			BundleVersion: mdocx.VersionV1,
			RootPath:      mdPath,
			Files: []mdocx.MarkdownFile{
				{Path: mdPath, Content: []byte(mdText)},
			},
		},
		Media: mdocx.MediaBundle{BundleVersion: mdocx.VersionV1},
	}

	if mediaFSPath != "" {
		b, err := os.ReadFile(mediaFSPath)
		if err != nil {
			log.Fatalf("read media: %v", err)
		}
		doc.Media.Items = append(doc.Media.Items, mdocx.MediaItem{
			ID:       mediaID,
			Path:     mediaContainerPath,
			MIMEType: mediaMIME,
			Data:     b,
		})
	}

	f, err := os.Create(outPath)
	if err != nil {
		log.Fatalf("create output: %v", err)
	}
	defer f.Close()

	if err := mdocx.Encode(f, doc); err != nil {
		log.Fatalf("encode: %v", err)
	}

	fmt.Printf("Wrote %s\n", outPath)
}
