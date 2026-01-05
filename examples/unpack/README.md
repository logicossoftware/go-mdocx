# unpack

Extracts an `.mdocx` file to an output folder:
- writes `metadata.json` if metadata is present
- writes markdown files at their container paths
- writes media at `MediaItem.Path` if present, otherwise `media/<ID>`

## Usage

```powershell
go run ./examples/unpack -in sample.mdocx -out outdir
```
