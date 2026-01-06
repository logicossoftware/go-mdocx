package mdocx

// readConfig holds configuration options for Decode.
type readConfig struct {
	limits       Limits
	verifyHashes bool
}

// ReadOption is a functional option for configuring Decode behavior.
type ReadOption func(*readConfig)

// WithReadLimits sets custom size limits for decoding.
// Zero values in l will be replaced with safe defaults.
//
// Example:
//
//	doc, err := mdocx.Decode(r, mdocx.WithReadLimits(mdocx.Limits{
//		MaxMediaUncompressed: 4 << 30, // Allow up to 4 GiB
//	}))
func WithReadLimits(l Limits) ReadOption {
	return func(c *readConfig) { c.limits = l }
}

// WithVerifyHashes controls whether non-zero MediaItem.SHA256 fields are verified during decode.
// When enabled (default), any SHA256 mismatch will cause Decode to return ErrValidation.
// Disable this for faster decoding when integrity has been verified externally.
func WithVerifyHashes(v bool) ReadOption {
	return func(c *readConfig) { c.verifyHashes = v }
}

// writeConfig holds configuration options for Encode.
type writeConfig struct {
	limits           Limits
	verifyHashes     bool
	autoPopulate     bool
	mdCompression    Compression
	mediaCompression Compression
}

// WriteOption is a functional option for configuring Encode behavior.
type WriteOption func(*writeConfig)

// WithWriteLimits sets custom size limits for encoding.
// Zero values in l will be replaced with safe defaults.
// Encoding will fail with ErrLimitExceeded if any limit is exceeded.
func WithWriteLimits(l Limits) WriteOption {
	return func(c *writeConfig) { c.limits = l }
}

// WithVerifyHashesOnWrite controls whether non-zero MediaItem.SHA256 fields are verified during encode.
// When enabled (default), any SHA256 mismatch will cause Encode to return ErrValidation.
// This verifies that provided hashes match the actual data before writing.
func WithVerifyHashesOnWrite(v bool) WriteOption {
	return func(c *writeConfig) { c.verifyHashes = v }
}

// WithAutoPopulateSHA256 controls whether Encode automatically computes SHA256 hashes
// for MediaItems that have a zero hash value.
// When enabled (default), doc.Media.Items will be modified in place to add computed hashes.
// Disable this if you need the document to remain unmodified.
func WithAutoPopulateSHA256(v bool) WriteOption {
	return func(c *writeConfig) { c.autoPopulate = v }
}

// WithMarkdownCompression sets the compression algorithm for the Markdown section.
// Default is CompZSTD. Use CompNone to disable compression.
//
// Compression selection guidance:
//   - CompZSTD: Recommended default, good speed/ratio balance
//   - CompZIP: Maximum interoperability with other tools
//   - CompLZ4: Maximum encode/decode speed
//   - CompBR: Maximum compression ratio (slower)
func WithMarkdownCompression(comp Compression) WriteOption {
	return func(c *writeConfig) { c.mdCompression = comp }
}

// WithMediaCompression sets the compression algorithm for the Media section.
// Default is CompZSTD. Use CompNone to disable compression.
// Note that media files (images, video) are often already compressed,
// so compression may not provide significant size reduction.
func WithMediaCompression(comp Compression) WriteOption {
	return func(c *writeConfig) { c.mediaCompression = comp }
}
