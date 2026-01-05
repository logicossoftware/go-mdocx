package mdocx

import "testing"

func TestValidateDocument_MoreFailures(t *testing.T) {
	l := defaultLimits()

	// Markdown bundle version wrong
	{
		d := sampleDoc()
		d.Markdown.BundleVersion = 2
		if err := validateDocument(d, l, false); err == nil {
			t.Fatal("expected error")
		}
	}
	// Markdown files empty
	{
		d := sampleDoc()
		d.Markdown.Files = nil
		if err := validateDocument(d, l, false); err == nil {
			t.Fatal("expected error")
		}
	}
	// Markdown RootPath invalid
	{
		d := sampleDoc()
		d.Markdown.RootPath = "/absolute/path.md"
		if err := validateDocument(d, l, false); err == nil {
			t.Fatal("expected error")
		}
	}
	// Media bundle version wrong
	{
		d := sampleDoc()
		d.Media.BundleVersion = 2
		if err := validateDocument(d, l, false); err == nil {
			t.Fatal("expected error")
		}
	}
	// Too many media items
	{
		l2 := l
		l2.MaxMediaItems = 0
		d := sampleDoc()
		if err := validateDocument(d, l2, false); err == nil {
			t.Fatal("expected error")
		}
	}
	// Media path invalid when set
	{
		d := sampleDoc()
		d.Media.Items[0].Path = "/abs.png"
		if err := validateDocument(d, l, false); err == nil {
			t.Fatal("expected error")
		}
	}
	// Markdown file too large
	{
		l2 := l
		l2.MaxSingleMarkdownFileSize = 1
		d := sampleDoc()
		if err := validateDocument(d, l2, false); err == nil {
			t.Fatal("expected error")
		}
	}
}
