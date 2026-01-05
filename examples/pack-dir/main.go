package main

import (
	"crypto/sha256"
	"flag"
	"fmt"
	"io/fs"
	"log"
	"mime"
	"os"
	"path/filepath"
	"sort"
	"strings"
	"time"

	"github.com/logicossoftware/go-mdocx"
)

func main() {
	var mdRoot string
	var mediaRoot string
	var outPath string
	var title string
	var rootMarkdown string

	flag.StringVar(&mdRoot, "md-root", "", "directory containing markdown files")
	flag.StringVar(&mediaRoot, "media-root", "", "directory containing media files")
	flag.StringVar(&outPath, "out", "bundle.mdocx", "output .mdocx file")
	flag.StringVar(&title, "title", "", "optional title metadata")
	flag.StringVar(&rootMarkdown, "root", "", "optional root markdown container path (must exist in bundle)")
	flag.Parse()

	if mdRoot == "" {
		log.Fatal("-md-root is required")
	}

	mdFiles, err := collectFiles(mdRoot, func(p string, d fs.DirEntry) bool {
		return !d.IsDir() && strings.HasSuffix(strings.ToLower(p), ".md")
	})
	if err != nil {
		log.Fatalf("collect markdown: %v", err)
	}
	if len(mdFiles) == 0 {
		log.Fatal("no markdown files found")
	}

	var markdown []mdocx.MarkdownFile
	for _, rel := range mdFiles {
		fsPath := filepath.Join(mdRoot, rel)
		b, err := os.ReadFile(fsPath)
		if err != nil {
			log.Fatalf("read %s: %v", fsPath, err)
		}
		containerPath := filepath.ToSlash(rel)
		markdown = append(markdown, mdocx.MarkdownFile{Path: containerPath, Content: b})
	}

	var mediaItems []mdocx.MediaItem
	if mediaRoot != "" {
		mediaFiles, err := collectFiles(mediaRoot, func(p string, d fs.DirEntry) bool {
			return !d.IsDir()
		})
		if err != nil {
			log.Fatalf("collect media: %v", err)
		}
		for _, rel := range mediaFiles {
			fsPath := filepath.Join(mediaRoot, rel)
			b, err := os.ReadFile(fsPath)
			if err != nil {
				log.Fatalf("read %s: %v", fsPath, err)
			}
			containerPath := filepath.ToSlash(rel)
			id := makeIDFromPath(containerPath)
			m := mime.TypeByExtension(filepath.Ext(fsPath))
			if m == "" {
				m = "application/octet-stream"
			}
			sum := sha256.Sum256(b)
			mediaItems = append(mediaItems, mdocx.MediaItem{
				ID:       id,
				Path:     containerPath,
				MIMEType: m,
				Data:     b,
				SHA256:   sum,
			})
		}
	}

	mdBundle := mdocx.MarkdownBundle{BundleVersion: mdocx.VersionV1, Files: markdown}
	if rootMarkdown != "" {
		mdBundle.RootPath = rootMarkdown
	}

	meta := map[string]any{
		"created_at": time.Now().UTC().Format(time.RFC3339),
	}
	if title != "" {
		meta["title"] = title
	}
	if rootMarkdown != "" {
		meta["root"] = rootMarkdown
	}

	doc := &mdocx.Document{
		Metadata: meta,
		Markdown: mdBundle,
		Media:    mdocx.MediaBundle{BundleVersion: mdocx.VersionV1, Items: mediaItems},
	}

	out, err := os.Create(outPath)
	if err != nil {
		log.Fatalf("create output: %v", err)
	}
	defer out.Close()

	if err := mdocx.Encode(out, doc); err != nil {
		log.Fatalf("encode: %v", err)
	}

	fmt.Printf("Packed %d markdown files and %d media items into %s\n", len(markdown), len(mediaItems), outPath)
}

func collectFiles(root string, keep func(rel string, d fs.DirEntry) bool) ([]string, error) {
	var out []string
	err := filepath.WalkDir(root, func(p string, d fs.DirEntry, err error) error {
		if err != nil {
			return err
		}
		if d.IsDir() {
			return nil
		}
		rel, err := filepath.Rel(root, p)
		if err != nil {
			return err
		}
		if keep(rel, d) {
			out = append(out, rel)
		}
		return nil
	})
	if err != nil {
		return nil, err
	}
	sort.Strings(out)
	return out, nil
}

func makeIDFromPath(p string) string {
	p = strings.ToLower(p)
	var b strings.Builder
	b.Grow(len(p))
	for _, r := range p {
		switch {
		case r >= 'a' && r <= 'z':
			b.WriteRune(r)
		case r >= '0' && r <= '9':
			b.WriteRune(r)
		default:
			b.WriteByte('_')
		}
	}
	id := b.String()
	id = strings.Trim(id, "_")
	if id == "" {
		return "media"
	}
	return id
}
