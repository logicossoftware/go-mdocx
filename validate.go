package mdocx

import (
	"crypto/subtle"
	"fmt"
	"path"
	"strings"
	"unicode/utf8"
)

func validateDocument(doc *Document, limits Limits, verifyHashes bool) error {
	if doc == nil {
		return fmt.Errorf("%w: document is nil", ErrValidation)
	}
	if doc.Markdown.BundleVersion != VersionV1 {
		return fmt.Errorf("%w: Markdown.BundleVersion must be %d", ErrValidation, VersionV1)
	}
	if len(doc.Markdown.Files) == 0 {
		return fmt.Errorf("%w: Markdown.Files must not be empty", ErrValidation)
	}
	if len(doc.Markdown.Files) > limits.MaxMarkdownFiles {
		return fmt.Errorf("%w: too many markdown files", ErrLimitExceeded)
	}
	// Validate RootPath if set
	if doc.Markdown.RootPath != "" {
		if err := validateContainerPath(doc.Markdown.RootPath); err != nil {
			return fmt.Errorf("%w: Markdown.RootPath: %v", ErrValidation, err)
		}
	}
	seenPaths := make(map[string]struct{}, len(doc.Markdown.Files))
	for i := range doc.Markdown.Files {
		f := doc.Markdown.Files[i]
		if err := validateContainerPath(f.Path); err != nil {
			return fmt.Errorf("%w: markdown file %d path: %v", ErrValidation, i, err)
		}
		if _, ok := seenPaths[f.Path]; ok {
			return fmt.Errorf("%w: duplicate markdown path %q", ErrValidation, f.Path)
		}
		seenPaths[f.Path] = struct{}{}
		if !utf8.Valid(f.Content) {
			return fmt.Errorf("%w: markdown file %q content is not valid UTF-8", ErrValidation, f.Path)
		}
		if uint64(len(f.Content)) > limits.MaxSingleMarkdownFileSize {
			return fmt.Errorf("%w: markdown file %q too large", ErrLimitExceeded, f.Path)
		}
	}
	if doc.Media.BundleVersion != VersionV1 {
		return fmt.Errorf("%w: Media.BundleVersion must be %d", ErrValidation, VersionV1)
	}
	if len(doc.Media.Items) > limits.MaxMediaItems {
		return fmt.Errorf("%w: too many media items", ErrLimitExceeded)
	}
	seenIDs := make(map[string]struct{}, len(doc.Media.Items))
	for i := range doc.Media.Items {
		it := doc.Media.Items[i]
		if strings.TrimSpace(it.ID) == "" {
			return fmt.Errorf("%w: media item %d has empty ID", ErrValidation, i)
		}
		if _, ok := seenIDs[it.ID]; ok {
			return fmt.Errorf("%w: duplicate media ID %q", ErrValidation, it.ID)
		}
		seenIDs[it.ID] = struct{}{}
		if it.Path != "" {
			if err := validateContainerPath(it.Path); err != nil {
				return fmt.Errorf("%w: media item %q path: %v", ErrValidation, it.ID, err)
			}
		}
		if uint64(len(it.Data)) > limits.MaxSingleMediaSize {
			return fmt.Errorf("%w: media item %q too large", ErrLimitExceeded, it.ID)
		}
		if verifyHashes {
			if it.SHA256 != ([32]byte{}) {
				computed := it.computedSHA256()
				if subtle.ConstantTimeCompare(computed[:], it.SHA256[:]) != 1 {
					return fmt.Errorf("%w: media item %q SHA256 mismatch", ErrValidation, it.ID)
				}
			}
		}
	}
	return nil
}

func validateContainerPath(p string) error {
	if strings.TrimSpace(p) == "" {
		return fmt.Errorf("path is empty")
	}
	if strings.HasPrefix(p, "/") {
		return fmt.Errorf("path must not be absolute")
	}
	if strings.Contains(p, "\\") {
		return fmt.Errorf("path must use forward slashes")
	}
	clean := path.Clean(p)
	if clean != p {
		return fmt.Errorf("path must be normalized: %q", clean)
	}
	if clean == "." {
		return fmt.Errorf("path must not be current directory")
	}
	if clean == ".." || strings.HasPrefix(clean, "../") {
		return fmt.Errorf("path must not escape")
	}
	return nil
}
