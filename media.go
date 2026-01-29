package mdocx

import (
	"io"
	"strings"
)

// ListMediaContents reads an MDOCX file and returns only the media items
// without fully decoding the markdown bundle. This is more efficient when
// you only need to access media contents.
//
// Example:
//
//	f, _ := os.Open("document.mdocx")
//	defer f.Close()
//	items, err := mdocx.ListMediaContents(f)
//	for _, item := range items {
//		fmt.Printf("%s: %s (%d bytes)\n", item.ID, item.MIMEType, len(item.Data))
//	}
func ListMediaContents(r io.Reader, opts ...ReadOption) ([]MediaItem, error) {
	doc, err := Decode(r, opts...)
	if err != nil {
		return nil, err
	}
	return doc.Media.Items, nil
}

// MediaResolver provides methods for resolving media references within a document.
// It supports lookups by ID, path, and mdocx:// URI references.
type MediaResolver struct {
	doc    *Document
	byID   map[string]*MediaItem
	byPath map[string]*MediaItem
	built  bool
}

// NewMediaResolver creates a new MediaResolver for the given document.
//
// Example:
//
//	resolver := mdocx.NewMediaResolver(doc)
//	item := resolver.GetByID("logo")
//	if item != nil {
//		fmt.Printf("Found logo: %s\n", item.MIMEType)
//	}
func NewMediaResolver(doc *Document) *MediaResolver {
	return &MediaResolver{doc: doc}
}

// build lazily constructs the lookup maps.
func (r *MediaResolver) build() {
	if r.built {
		return
	}
	r.byID = make(map[string]*MediaItem, len(r.doc.Media.Items))
	r.byPath = make(map[string]*MediaItem, len(r.doc.Media.Items))
	for i := range r.doc.Media.Items {
		item := &r.doc.Media.Items[i]
		r.byID[item.ID] = item
		if item.Path != "" {
			r.byPath[item.Path] = item
		}
	}
	r.built = true
}

// GetByID returns the media item with the given ID, or nil if not found.
func (r *MediaResolver) GetByID(id string) *MediaItem {
	r.build()
	return r.byID[id]
}

// GetByPath returns the media item with the given container path, or nil if not found.
func (r *MediaResolver) GetByPath(path string) *MediaItem {
	r.build()
	return r.byPath[path]
}

// HasID returns true if a media item with the given ID exists.
func (r *MediaResolver) HasID(id string) bool {
	r.build()
	_, ok := r.byID[id]
	return ok
}

// HasPath returns true if a media item with the given path exists.
func (r *MediaResolver) HasPath(path string) bool {
	r.build()
	_, ok := r.byPath[path]
	return ok
}

// Resolve resolves a media reference string to a MediaItem.
// It supports:
//   - mdocx://media/<ID> URIs (resolved by ID)
//   - Relative paths (resolved by Path)
//
// Returns nil if the reference cannot be resolved.
//
// Example:
//
//	item := resolver.Resolve("mdocx://media/logo")
//	item := resolver.Resolve("assets/logo.png")
func (r *MediaResolver) Resolve(ref string) *MediaItem {
	r.build()

	// Check for mdocx://media/<id> URI scheme
	if strings.HasPrefix(ref, "mdocx://media/") {
		id := strings.TrimPrefix(ref, "mdocx://media/")
		return r.byID[id]
	}

	// Try as path reference
	if item, ok := r.byPath[ref]; ok {
		return item
	}

	// Try as ID (for cases like bare IDs)
	return r.byID[ref]
}

// GetReferencedMedia returns all media items referenced by a markdown file.
// It uses the MarkdownFile.MediaRefs field to find referenced items.
func (r *MediaResolver) GetReferencedMedia(file *MarkdownFile) []*MediaItem {
	if file == nil || len(file.MediaRefs) == 0 {
		return nil
	}
	r.build()

	items := make([]*MediaItem, 0, len(file.MediaRefs))
	for _, ref := range file.MediaRefs {
		if item, ok := r.byID[ref]; ok {
			items = append(items, item)
		}
	}
	return items
}

// All returns all media items in the document.
func (r *MediaResolver) All() []MediaItem {
	return r.doc.Media.Items
}

// Count returns the number of media items in the document.
func (r *MediaResolver) Count() int {
	return len(r.doc.Media.Items)
}

// IDs returns all media item IDs.
func (r *MediaResolver) IDs() []string {
	ids := make([]string, len(r.doc.Media.Items))
	for i, item := range r.doc.Media.Items {
		ids[i] = item.ID
	}
	return ids
}

// Paths returns all non-empty media item paths.
func (r *MediaResolver) Paths() []string {
	paths := make([]string, 0, len(r.doc.Media.Items))
	for _, item := range r.doc.Media.Items {
		if item.Path != "" {
			paths = append(paths, item.Path)
		}
	}
	return paths
}

// MediaReference represents a parsed media reference.
type MediaReference struct {
	// Type is either "id" or "path".
	Type string
	// ID is the media ID (set when Type is "id").
	ID string
	// Path is the container path (set when Type is "path").
	Path string
}

// ParseMediaReference parses a reference string into a MediaReference.
//
// Example:
//
//	ref := mdocx.ParseMediaReference("mdocx://media/logo")
//	// ref.Type == "id", ref.ID == "logo"
//
//	ref := mdocx.ParseMediaReference("assets/image.png")
//	// ref.Type == "path", ref.Path == "assets/image.png"
func ParseMediaReference(ref string) MediaReference {
	if strings.HasPrefix(ref, "mdocx://media/") {
		return MediaReference{
			Type: "id",
			ID:   strings.TrimPrefix(ref, "mdocx://media/"),
		}
	}
	return MediaReference{
		Type: "path",
		Path: ref,
	}
}
