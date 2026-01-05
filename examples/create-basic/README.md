# create-basic

Creates a minimal `.mdocx` file with:
- metadata JSON
- one markdown file
- optional one media item

## Usage

```powershell
go run ./examples/create-basic -out sample.mdocx -title "Hello" -md docs/index.md -media assets/logo.png -media-id logo_png -media-mime image/png
```

- `-md` is a container path (e.g. `docs/index.md`), not a filesystem path.
- `-media` reads bytes from the filesystem path, but stores as `-media-path` inside the container.
