package main

import (
	"encoding/json"
	"flag"
	"fmt"
	"log"
	"os"
	"path/filepath"
	"strings"

	"github.com/logicossoftware/go-mdocx"
)

func main() {
	var inPath string
	var outDir string
	flag.StringVar(&inPath, "in", "", "input .mdocx file")
	flag.StringVar(&outDir, "out", "out", "output directory")
	flag.Parse()
	if inPath == "" {
		log.Fatal("-in is required")
	}

	f, err := os.Open(inPath)
	if err != nil {
		log.Fatalf("open: %v", err)
	}
	defer f.Close()

	doc, err := mdocx.Decode(f)
	if err != nil {
		log.Fatalf("decode: %v", err)
	}

	if err := os.MkdirAll(outDir, 0o755); err != nil {
		log.Fatalf("mkdir: %v", err)
	}

	if doc.Metadata != nil {
		b, err := json.MarshalIndent(doc.Metadata, "", "  ")
		if err != nil {
			log.Fatalf("metadata json: %v", err)
		}
		p := filepath.Join(outDir, "metadata.json")
		if err := os.WriteFile(p, b, 0o644); err != nil {
			log.Fatalf("write metadata: %v", err)
		}
		fmt.Printf("wrote %s\n", p)
	}

	for _, mf := range doc.Markdown.Files {
		p := filepath.Join(outDir, filepath.FromSlash(mf.Path))
		if err := os.MkdirAll(filepath.Dir(p), 0o755); err != nil {
			log.Fatalf("mkdir: %v", err)
		}
		if err := os.WriteFile(p, mf.Content, 0o644); err != nil {
			log.Fatalf("write markdown: %v", err)
		}
		fmt.Printf("wrote %s\n", p)
	}

	for _, mi := range doc.Media.Items {
		containerPath := mi.Path
		if strings.TrimSpace(containerPath) == "" {
			containerPath = "media/" + mi.ID
		}
		p := filepath.Join(outDir, filepath.FromSlash(containerPath))
		if err := os.MkdirAll(filepath.Dir(p), 0o755); err != nil {
			log.Fatalf("mkdir: %v", err)
		}
		if err := os.WriteFile(p, mi.Data, 0o644); err != nil {
			log.Fatalf("write media: %v", err)
		}
		fmt.Printf("wrote %s\n", p)
	}
}
