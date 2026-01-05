package mdocx

type readConfig struct {
	limits       Limits
	verifyHashes bool
}

type ReadOption func(*readConfig)

func WithReadLimits(l Limits) ReadOption {
	return func(c *readConfig) { c.limits = l }
}

func WithVerifyHashes(v bool) ReadOption {
	return func(c *readConfig) { c.verifyHashes = v }
}

type writeConfig struct {
	limits           Limits
	verifyHashes     bool
	autoPopulate     bool
	mdCompression    Compression
	mediaCompression Compression
}

type WriteOption func(*writeConfig)

func WithWriteLimits(l Limits) WriteOption {
	return func(c *writeConfig) { c.limits = l }
}

// WithVerifyHashes controls whether non-zero MediaItem.SHA256 fields are verified on Encode.
// Decode always uses WithVerifyHashes.
func WithVerifyHashesOnWrite(v bool) WriteOption {
	return func(c *writeConfig) { c.verifyHashes = v }
}

// WithAutoPopulateSHA256 causes Encode to compute SHA256 for items with a zero hash.
func WithAutoPopulateSHA256(v bool) WriteOption {
	return func(c *writeConfig) { c.autoPopulate = v }
}

func WithMarkdownCompression(comp Compression) WriteOption {
	return func(c *writeConfig) { c.mdCompression = comp }
}

func WithMediaCompression(comp Compression) WriteOption {
	return func(c *writeConfig) { c.mediaCompression = comp }
}
