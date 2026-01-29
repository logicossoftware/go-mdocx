package mdocx

import (
	"bytes"
	"crypto/sha256"
	"testing"
)

func TestMediaResolver_GetByID(t *testing.T) {
	doc := &Document{
		Markdown: MarkdownBundle{
			BundleVersion: VersionV1,
			Files:         []MarkdownFile{{Path: "readme.md", Content: []byte("# Test")}},
		},
		Media: MediaBundle{
			BundleVersion: VersionV1,
			Items: []MediaItem{
				{ID: "logo", Path: "assets/logo.png", MIMEType: "image/png", Data: []byte{1, 2, 3}},
				{ID: "icon", Path: "assets/icon.svg", MIMEType: "image/svg+xml", Data: []byte{4, 5, 6}},
			},
		},
	}

	resolver := NewMediaResolver(doc)

	t.Run("existing ID", func(t *testing.T) {
		item := resolver.GetByID("logo")
		if item == nil {
			t.Fatal("expected item, got nil")
		}
		if item.ID != "logo" {
			t.Errorf("expected ID 'logo', got %q", item.ID)
		}
		if item.MIMEType != "image/png" {
			t.Errorf("expected MIMEType 'image/png', got %q", item.MIMEType)
		}
	})

	t.Run("non-existing ID", func(t *testing.T) {
		item := resolver.GetByID("nonexistent")
		if item != nil {
			t.Errorf("expected nil, got %v", item)
		}
	})
}

func TestMediaResolver_GetByPath(t *testing.T) {
	doc := &Document{
		Markdown: MarkdownBundle{
			BundleVersion: VersionV1,
			Files:         []MarkdownFile{{Path: "readme.md", Content: []byte("# Test")}},
		},
		Media: MediaBundle{
			BundleVersion: VersionV1,
			Items: []MediaItem{
				{ID: "logo", Path: "assets/logo.png", MIMEType: "image/png", Data: []byte{1, 2, 3}},
				{ID: "icon", MIMEType: "image/svg+xml", Data: []byte{4, 5, 6}}, // no path
			},
		},
	}

	resolver := NewMediaResolver(doc)

	t.Run("existing path", func(t *testing.T) {
		item := resolver.GetByPath("assets/logo.png")
		if item == nil {
			t.Fatal("expected item, got nil")
		}
		if item.ID != "logo" {
			t.Errorf("expected ID 'logo', got %q", item.ID)
		}
	})

	t.Run("non-existing path", func(t *testing.T) {
		item := resolver.GetByPath("nonexistent.png")
		if item != nil {
			t.Errorf("expected nil, got %v", item)
		}
	})
}

func TestMediaResolver_Resolve(t *testing.T) {
	doc := &Document{
		Markdown: MarkdownBundle{
			BundleVersion: VersionV1,
			Files:         []MarkdownFile{{Path: "readme.md", Content: []byte("# Test")}},
		},
		Media: MediaBundle{
			BundleVersion: VersionV1,
			Items: []MediaItem{
				{ID: "logo", Path: "assets/logo.png", MIMEType: "image/png", Data: []byte{1, 2, 3}},
				{ID: "icon", Path: "assets/icon.svg", MIMEType: "image/svg+xml", Data: []byte{4, 5, 6}},
			},
		},
	}

	resolver := NewMediaResolver(doc)

	tests := []struct {
		name    string
		ref     string
		wantID  string
		wantNil bool
	}{
		{"mdocx URI", "mdocx://media/logo", "logo", false},
		{"mdocx URI icon", "mdocx://media/icon", "icon", false},
		{"path reference", "assets/logo.png", "logo", false},
		{"bare ID fallback", "icon", "icon", false},
		{"nonexistent URI", "mdocx://media/missing", "", true},
		{"nonexistent path", "missing.png", "", true},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			item := resolver.Resolve(tt.ref)
			if tt.wantNil {
				if item != nil {
					t.Errorf("expected nil, got %v", item)
				}
				return
			}
			if item == nil {
				t.Fatal("expected item, got nil")
			}
			if item.ID != tt.wantID {
				t.Errorf("expected ID %q, got %q", tt.wantID, item.ID)
			}
		})
	}
}

func TestMediaResolver_HasID(t *testing.T) {
	doc := &Document{
		Markdown: MarkdownBundle{BundleVersion: VersionV1, Files: []MarkdownFile{{Path: "a.md", Content: []byte("#")}}},
		Media: MediaBundle{
			BundleVersion: VersionV1,
			Items:         []MediaItem{{ID: "exists", Data: []byte{1}}},
		},
	}

	resolver := NewMediaResolver(doc)

	if !resolver.HasID("exists") {
		t.Error("expected HasID('exists') to be true")
	}
	if resolver.HasID("missing") {
		t.Error("expected HasID('missing') to be false")
	}
}

func TestMediaResolver_HasPath(t *testing.T) {
	doc := &Document{
		Markdown: MarkdownBundle{BundleVersion: VersionV1, Files: []MarkdownFile{{Path: "a.md", Content: []byte("#")}}},
		Media: MediaBundle{
			BundleVersion: VersionV1,
			Items:         []MediaItem{{ID: "img", Path: "assets/img.png", Data: []byte{1}}},
		},
	}

	resolver := NewMediaResolver(doc)

	if !resolver.HasPath("assets/img.png") {
		t.Error("expected HasPath('assets/img.png') to be true")
	}
	if resolver.HasPath("missing.png") {
		t.Error("expected HasPath('missing.png') to be false")
	}
}

func TestMediaResolver_GetReferencedMedia(t *testing.T) {
	doc := &Document{
		Markdown: MarkdownBundle{
			BundleVersion: VersionV1,
			Files: []MarkdownFile{
				{Path: "readme.md", Content: []byte("# Test"), MediaRefs: []string{"logo", "icon"}},
				{Path: "other.md", Content: []byte("# Other"), MediaRefs: []string{"logo"}},
				{Path: "norefs.md", Content: []byte("# No Refs")},
			},
		},
		Media: MediaBundle{
			BundleVersion: VersionV1,
			Items: []MediaItem{
				{ID: "logo", Data: []byte{1}},
				{ID: "icon", Data: []byte{2}},
				{ID: "unused", Data: []byte{3}},
			},
		},
	}

	resolver := NewMediaResolver(doc)

	t.Run("file with two refs", func(t *testing.T) {
		items := resolver.GetReferencedMedia(&doc.Markdown.Files[0])
		if len(items) != 2 {
			t.Fatalf("expected 2 items, got %d", len(items))
		}
		ids := make(map[string]bool)
		for _, item := range items {
			ids[item.ID] = true
		}
		if !ids["logo"] || !ids["icon"] {
			t.Errorf("expected logo and icon, got %v", ids)
		}
	})

	t.Run("file with one ref", func(t *testing.T) {
		items := resolver.GetReferencedMedia(&doc.Markdown.Files[1])
		if len(items) != 1 {
			t.Fatalf("expected 1 item, got %d", len(items))
		}
		if items[0].ID != "logo" {
			t.Errorf("expected logo, got %s", items[0].ID)
		}
	})

	t.Run("file with no refs", func(t *testing.T) {
		items := resolver.GetReferencedMedia(&doc.Markdown.Files[2])
		if items != nil {
			t.Errorf("expected nil, got %v", items)
		}
	})

	t.Run("nil file", func(t *testing.T) {
		items := resolver.GetReferencedMedia(nil)
		if items != nil {
			t.Errorf("expected nil, got %v", items)
		}
	})
}

func TestMediaResolver_All(t *testing.T) {
	doc := &Document{
		Markdown: MarkdownBundle{BundleVersion: VersionV1, Files: []MarkdownFile{{Path: "a.md", Content: []byte("#")}}},
		Media: MediaBundle{
			BundleVersion: VersionV1,
			Items: []MediaItem{
				{ID: "a", Data: []byte{1}},
				{ID: "b", Data: []byte{2}},
			},
		},
	}

	resolver := NewMediaResolver(doc)
	all := resolver.All()

	if len(all) != 2 {
		t.Fatalf("expected 2 items, got %d", len(all))
	}
}

func TestMediaResolver_Count(t *testing.T) {
	doc := &Document{
		Markdown: MarkdownBundle{BundleVersion: VersionV1, Files: []MarkdownFile{{Path: "a.md", Content: []byte("#")}}},
		Media: MediaBundle{
			BundleVersion: VersionV1,
			Items:         []MediaItem{{ID: "a", Data: []byte{1}}, {ID: "b", Data: []byte{2}}, {ID: "c", Data: []byte{3}}},
		},
	}

	resolver := NewMediaResolver(doc)
	if resolver.Count() != 3 {
		t.Errorf("expected 3, got %d", resolver.Count())
	}
}

func TestMediaResolver_IDs(t *testing.T) {
	doc := &Document{
		Markdown: MarkdownBundle{BundleVersion: VersionV1, Files: []MarkdownFile{{Path: "a.md", Content: []byte("#")}}},
		Media: MediaBundle{
			BundleVersion: VersionV1,
			Items:         []MediaItem{{ID: "alpha", Data: []byte{1}}, {ID: "beta", Data: []byte{2}}},
		},
	}

	resolver := NewMediaResolver(doc)
	ids := resolver.IDs()

	if len(ids) != 2 {
		t.Fatalf("expected 2 IDs, got %d", len(ids))
	}
	// Order should be preserved
	if ids[0] != "alpha" || ids[1] != "beta" {
		t.Errorf("expected [alpha, beta], got %v", ids)
	}
}

func TestMediaResolver_Paths(t *testing.T) {
	doc := &Document{
		Markdown: MarkdownBundle{BundleVersion: VersionV1, Files: []MarkdownFile{{Path: "a.md", Content: []byte("#")}}},
		Media: MediaBundle{
			BundleVersion: VersionV1,
			Items: []MediaItem{
				{ID: "a", Path: "assets/a.png", Data: []byte{1}},
				{ID: "b", Data: []byte{2}}, // no path
				{ID: "c", Path: "assets/c.png", Data: []byte{3}},
			},
		},
	}

	resolver := NewMediaResolver(doc)
	paths := resolver.Paths()

	if len(paths) != 2 {
		t.Fatalf("expected 2 paths, got %d", len(paths))
	}
	if paths[0] != "assets/a.png" || paths[1] != "assets/c.png" {
		t.Errorf("expected [assets/a.png, assets/c.png], got %v", paths)
	}
}

func TestParseMediaReference(t *testing.T) {
	tests := []struct {
		input    string
		wantType string
		wantID   string
		wantPath string
	}{
		{"mdocx://media/logo", "id", "logo", ""},
		{"mdocx://media/my-image-123", "id", "my-image-123", ""},
		{"assets/logo.png", "path", "", "assets/logo.png"},
		{"image.jpg", "path", "", "image.jpg"},
		{"deep/nested/path/file.svg", "path", "", "deep/nested/path/file.svg"},
	}

	for _, tt := range tests {
		t.Run(tt.input, func(t *testing.T) {
			ref := ParseMediaReference(tt.input)
			if ref.Type != tt.wantType {
				t.Errorf("Type: expected %q, got %q", tt.wantType, ref.Type)
			}
			if ref.ID != tt.wantID {
				t.Errorf("ID: expected %q, got %q", tt.wantID, ref.ID)
			}
			if ref.Path != tt.wantPath {
				t.Errorf("Path: expected %q, got %q", tt.wantPath, ref.Path)
			}
		})
	}
}

func TestListMediaContents(t *testing.T) {
	// Create a document with media
	data1 := []byte{1, 2, 3, 4, 5}
	data2 := []byte{6, 7, 8, 9, 10}
	hash1 := sha256.Sum256(data1)
	hash2 := sha256.Sum256(data2)

	doc := &Document{
		Markdown: MarkdownBundle{
			BundleVersion: VersionV1,
			Files:         []MarkdownFile{{Path: "readme.md", Content: []byte("# Test")}},
		},
		Media: MediaBundle{
			BundleVersion: VersionV1,
			Items: []MediaItem{
				{ID: "img1", Path: "a.png", MIMEType: "image/png", Data: data1, SHA256: hash1},
				{ID: "img2", Path: "b.png", MIMEType: "image/png", Data: data2, SHA256: hash2},
			},
		},
	}

	// Encode to bytes
	var buf bytes.Buffer
	if err := Encode(&buf, doc); err != nil {
		t.Fatalf("Encode failed: %v", err)
	}

	// List media contents
	items, err := ListMediaContents(&buf)
	if err != nil {
		t.Fatalf("ListMediaContents failed: %v", err)
	}

	if len(items) != 2 {
		t.Fatalf("expected 2 items, got %d", len(items))
	}

	if items[0].ID != "img1" || items[1].ID != "img2" {
		t.Errorf("unexpected items: %v", items)
	}

	if !bytes.Equal(items[0].Data, data1) {
		t.Error("data1 mismatch")
	}
	if !bytes.Equal(items[1].Data, data2) {
		t.Error("data2 mismatch")
	}
}

func TestListMediaContents_Empty(t *testing.T) {
	doc := &Document{
		Markdown: MarkdownBundle{
			BundleVersion: VersionV1,
			Files:         []MarkdownFile{{Path: "readme.md", Content: []byte("# Test")}},
		},
		Media: MediaBundle{BundleVersion: VersionV1},
	}

	var buf bytes.Buffer
	if err := Encode(&buf, doc); err != nil {
		t.Fatalf("Encode failed: %v", err)
	}

	items, err := ListMediaContents(&buf)
	if err != nil {
		t.Fatalf("ListMediaContents failed: %v", err)
	}

	if len(items) != 0 {
		t.Errorf("expected 0 items, got %d", len(items))
	}
}
