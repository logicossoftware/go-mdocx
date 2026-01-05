# pack-dir

Packs a directory into an `.mdocx` file.

- Markdown files: all `*.md` under `-md-root`.
- Media files: all files under `-media-root`.

Container paths are computed relative to their root and stored with forward slashes.
Media IDs are derived deterministically from their container paths.

## Usage

```powershell
go run ./examples/pack-dir -md-root .\docs -media-root .\assets -out bundle.mdocx -title "My Bundle"
```
