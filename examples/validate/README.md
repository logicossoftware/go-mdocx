# validate

A command-line tool for validating MDOCX files and generating test suites. This tool is essential for testing MDOCX implementations in other programming languages against the Go reference implementation.

## Usage

### Validation Mode

```bash
validate -in <file.mdocx> [-details] [-preview] [-preview-len N]
```

### Test Suite Generation Mode

```bash
validate -generate-test-suite <output-directory>
```

## Flags

| Flag | Description |
|------|-------------|
| `-in` | Input .mdocx file to validate |
| `-details` | Include detailed file/media information |
| `-preview` | Include content preview for markdown files (requires `-details`) |
| `-preview-len` | Maximum length of content preview (default: 200) |
| `-generate-test-suite` | Generate test suite files in specified directory |

## Validation Output Format

The tool outputs JSON to stdout. Exit code is 0 for valid files, 1 for invalid files or errors.

### Basic Validation

```json
{
  "valid": true,
  "header": {
    "magic_hex": "4d444f43580d0a1a",
    "magic_valid": true,
    "version": 1,
    "header_flags": 1,
    "fixed_header_size": 32,
    "metadata_length": 85
  },
  "summary": {
    "has_metadata": true,
    "markdown_file_count": 2,
    "media_item_count": 1,
    "total_markdown_bytes": 1234,
    "total_media_bytes": 5678
  }
}
```

### With Details (`-details`)

```json
{
  "valid": true,
  "header": { ... },
  "summary": { ... },
  "details": {
    "metadata": {
      "title": "Example Document",
      "created_at": "2026-01-05T12:00:00Z"
    },
    "markdown_files": [
      {
        "path": "docs/index.md",
        "content_length": 150,
        "content_sha256": "abcd1234...",
        "media_refs": ["logo"],
        "attributes": null
      }
    ],
    "media_items": [
      {
        "id": "logo",
        "path": "assets/logo.png",
        "mime_type": "image/png",
        "data_length": 5678,
        "sha256_stored": "ef567890...",
        "sha256_computed": "ef567890...",
        "sha256_valid": true,
        "attributes": null
      }
    ]
  }
}
```

## Test Suite Generation

Generate a comprehensive test suite for cross-language testing:

```bash
validate -generate-test-suite ./testsuite
```

This creates:

### Generated Files

| File | Description |
|------|-------------|
| `minimal_uncompressed.mdocx` | Minimal valid file, no compression |
| `minimal_zstd.mdocx` | ZSTD compression |
| `minimal_zip.mdocx` | ZIP compression |
| `minimal_lz4.mdocx` | LZ4 compression |
| `minimal_brotli.mdocx` | Brotli compression |
| `with_metadata.mdocx` | Full metadata block |
| `multi_markdown.mdocx` | Multiple markdown files |
| `with_media.mdocx` | Media items with SHA256 |
| `full_featured.mdocx` | All features combined |
| `media_refs.mdocx` | mdocx:// URI references |
| `unicode_content.mdocx` | Unicode/emoji content |
| `empty_media_bundle.mdocx` | Empty media bundle |
| `attributes.mdocx` | Custom attributes |
| `deep_paths.mdocx` | Nested file paths |
| `large_content.mdocx` | Larger payloads |

### Expected Output Files

For each `.mdocx` file, a corresponding `.mdocx.expected.json` file is generated containing the expected validation output with full details.

### Manifest

A `manifest.json` file describes all test cases:

```json
{
  "description": "MDOCX v1 test suite for cross-language implementation testing",
  "files": [
    {
      "filename": "minimal_uncompressed.mdocx",
      "description": "Minimal valid file with no compression, no metadata, no media",
      "compression": "none",
      "has_metadata": false,
      "has_media": false,
      "markdown_file_count": 1,
      "media_item_count": 0
    },
    ...
  ]
}
```

## Cross-Language Testing Workflow

1. **Generate the test suite** using the Go reference implementation:
   ```bash
   validate -generate-test-suite ./testsuite
   ```

2. **Implement your MDOCX parser** in your target language

3. **For each test file**, parse it and compare your output against the `.expected.json` file

4. **Verify**:
   - Header parsing (magic, version, flags)
   - Metadata JSON parsing
   - Markdown bundle decoding (all compression types)
   - Media bundle decoding
   - SHA256 hash verification
   - Unicode handling
   - Path handling

## Example: CI Testing

```bash
#!/bin/bash
# Generate reference test suite
go run ./examples/validate -generate-test-suite ./testsuite

# Run your implementation's test suite against generated files
your-implementation test ./testsuite

# Validate specific file
go run ./examples/validate -in output.mdocx -details
```
