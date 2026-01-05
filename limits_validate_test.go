package mdocx

import "testing"

func TestLimitsWithDefaults(t *testing.T) {
	l := (Limits{}).withDefaults()
	if l.MaxMetadataLen == 0 || l.MaxMarkdownUncompressed == 0 || l.MaxMediaUncompressed == 0 {
		t.Fatal("expected defaults")
	}

	custom := Limits{MaxMetadataLen: 7}
	custom = custom.withDefaults()
	if custom.MaxMetadataLen != 7 {
		t.Fatalf("expected custom MaxMetadataLen, got %d", custom.MaxMetadataLen)
	}
}

func TestValidateContainerPath(t *testing.T) {
	cases := []struct {
		in   string
		want bool
	}{
		{"docs/readme.md", true},
		{"", false},
		{"/abs", false},
		{"a\\b", false},
		{"a//b", false},
		{"a/./b", false},
		{"a/../b", false},
		{".", false},
		{"..", false},
		{"../x", false},
		{"a/", false},
	}
	for _, tc := range cases {
		err := validateContainerPath(tc.in)
		if tc.want && err != nil {
			t.Fatalf("%q: expected ok, got %v", tc.in, err)
		}
		if !tc.want && err == nil {
			t.Fatalf("%q: expected error", tc.in)
		}
	}
}

func TestCompressionNameUnknown(t *testing.T) {
	if compressionName(Compression(99)) != "unknown" {
		t.Fatal("expected unknown")
	}
}
