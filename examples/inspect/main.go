package main

import (
	"encoding/json"
	"flag"
	"fmt"
	"log"
	"os"
	"sort"

	"github.com/logicossoftware/go-mdocx"
)

type summary struct {
	MetadataKeys  []string `json:"metadata_keys"`
	MarkdownFiles []string `json:"markdown_files"`
	MediaIDs      []string `json:"media_ids"`
	MediaPaths    []string `json:"media_paths"`
}

func main() {
	var inPath string
	flag.StringVar(&inPath, "in", "", "input .mdocx file")
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

	var s summary
	for k := range doc.Metadata {
		s.MetadataKeys = append(s.MetadataKeys, k)
	}
	for _, mf := range doc.Markdown.Files {
		s.MarkdownFiles = append(s.MarkdownFiles, mf.Path)
	}
	for _, mi := range doc.Media.Items {
		s.MediaIDs = append(s.MediaIDs, mi.ID)
		if mi.Path != "" {
			s.MediaPaths = append(s.MediaPaths, mi.Path)
		}
	}
	sort.Strings(s.MetadataKeys)
	sort.Strings(s.MarkdownFiles)
	sort.Strings(s.MediaIDs)
	sort.Strings(s.MediaPaths)

	b, _ := json.MarshalIndent(s, "", "  ")
	fmt.Println(string(b))
}
