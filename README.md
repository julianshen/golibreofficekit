# golibreofficekit

Go binding for [LibreOfficeKit](https://wiki.documentfoundation.org/Development/LibreOfficeKit)
(LOK), the C ABI exposed by LibreOffice for in-process loading, rendering,
editing, and saving of Writer, Calc, Impress, and Draw documents.

**Status:** 1.0. The public API in [`lok`](./lok) is stable under SemVer;
breaking changes will bump the major version. See [`CHANGELOG.md`](./CHANGELOG.md)
for release notes and the [`lok` package godoc](https://pkg.go.dev/github.com/julianshen/golibreofficekit/lok)
for the current API surface.

## What it does

- Load `.odt` / `.docx`, `.ods` / `.xlsx`, `.odp` / `.pptx`, `.odg`, and
  every other format LibreOffice can open.
- Save into any LO export filter (PDF, PNG, DOCX, XLSX, PPTX, plain text,
  Markdown, …).
- Render any page or the whole document to a BGRA bitmap or PNG at an
  arbitrary DPI scale.
- Drive editing — key/mouse events, UNO commands, text/graphic selection,
  clipboard, multiple views per document.
- Subscribe to LO callbacks (selection changed, invalidations, save status,
  …) with bounded per-listener queues; dropped-event and panicked-listener
  counters are exposed so consumer bugs are observable.
- Surface LibreOffice's own error strings (`getError()`) when load/save
  fails so callers see "password required" / "filter rejected file"
  instead of generic "documentLoad returned NULL".
- Detect missing LOK vtable slots on stripped LibreOffice builds by
  returning `ErrUnsupported` rather than silently no-opping.

## Quickstart

```go
import "github.com/julianshen/golibreofficekit/lok"

office, err := lok.New("/usr/lib64/libreoffice/program")
if err != nil {
    return err
}
defer office.Close()

doc, err := office.Load("report.docx")
if err != nil {
    return err
}
defer doc.Close()

// Convert to PDF
if err := doc.SaveAs("report.pdf", "pdf", ""); err != nil {
    return err
}

// Render the first page as PNG at 1.5× DPI
if err := doc.InitializeForRendering(""); err != nil {
    return err
}
png, err := doc.RenderPagePNG(0, 1.5)
if err != nil { return err }
_ = os.WriteFile("page-1.png", png, 0o644)
```

See the [`lok` package godoc](https://pkg.go.dev/github.com/julianshen/golibreofficekit/lok)
for concepts (Office singleton, per-document mutex, listener model),
threading rules, and the full error sentinel list.

## Command-line tools

The repo ships two CLI examples that double as integration smoke tests.

### `cmd/lokconv` — convert documents to PDF or PNG

```bash
go install github.com/julianshen/golibreofficekit/cmd/lokconv@latest

lokconv -in report.docx -out report.pdf
lokconv -in deck.pptx  -out slide-2.png -page 1 -dpi 1.5
```

Output format is inferred from the `-out` extension (`.pdf` or `.png`).

### `cmd/lokmd` — Markdown ↔ DOCX/PPTX

```bash
go install github.com/julianshen/golibreofficekit/cmd/lokmd@latest

# Round-trip Markdown notes through Word
lokmd -in notes.md   -out notes.docx
lokmd -in notes.docx -out notes.md

# Marp-style deck → PowerPoint
lokmd -in deck.md  -out deck.pptx
lokmd -in deck.pptx -out deck.md
```

The Markdown side follows [Marp/Marpit](https://marpit.marp.app/)
conventions: `---` on its own line separates slides, a leading YAML
front-matter block (between two `---`) is stripped, and the first
`# ` heading per slide becomes the slide title.

Both CLIs read the LibreOffice install path from `-lo-path`, then
`$LOK_PATH`, then a small list of platform-default candidates.

## Installation

### Prerequisites

- **Go** 1.23 or newer.
- **LibreOffice** 24.8 or newer (7.6+ usually works for basic
  load/save/render paths).
- **Platform**: Linux (x86_64 / aarch64) or macOS (x86_64 / arm64).
  Windows is not supported.

The cgo build is self-contained — the LOK header is vendored under
`third_party/lok/`, so no `-dev` / `-devel` package is required.

### Install LibreOffice (Linux)

**Fedora / RHEL / CentOS Stream:**

```bash
sudo dnf install libreoffice
# or, for a smaller footprint without the GUI front-end:
sudo dnf install libreoffice-core libreoffice-writer libreoffice-calc \
                 libreoffice-impress libreoffice-draw
```

**Debian / Ubuntu:**

```bash
sudo apt update
sudo apt install libreoffice
```

**Arch Linux:**

```bash
sudo pacman -S libreoffice-fresh    # or libreoffice-still for the LTS branch
```

**openSUSE:**

```bash
sudo zypper install libreoffice
```

### Install LibreOffice (macOS)

**Homebrew (recommended):**

```bash
brew install --cask libreoffice
```

**Direct download:** install the `.dmg` from
<https://www.libreoffice.org/download/>.

### Locate the install path

`lok.New` (and the CLIs' `-lo-path` flag / `$LOK_PATH` env var) need the
absolute path of LibreOffice's `program/` directory:

| Platform                   | Typical path                                            |
|----------------------------|---------------------------------------------------------|
| Fedora / RHEL              | `/usr/lib64/libreoffice/program`                        |
| Debian / Ubuntu            | `/usr/lib/libreoffice/program`                          |
| Arch / openSUSE            | `/usr/lib/libreoffice/program`                          |
| macOS (Homebrew or direct) | `/Applications/LibreOffice.app/Contents/Frameworks`     |

Verify the path is correct — one of the LOK shared libraries must exist
inside it:

```bash
# Linux: either libsofficeapp.so or libmergedlo.so (Debian/Ubuntu's apt build)
ls "$LOK_PATH"/libsofficeapp.so "$LOK_PATH"/libmergedlo.so 2>/dev/null

# macOS:
ls "$LOK_PATH"/libsofficeapp.dylib
```

For convenience, export `LOK_PATH` so the binding and the CLIs pick it
up automatically:

```bash
# Linux (~/.bashrc or ~/.zshrc):
export LOK_PATH=/usr/lib64/libreoffice/program

# macOS:
export LOK_PATH=/Applications/LibreOffice.app/Contents/Frameworks
```

If the path is wrong, `lok.New` returns an error wrapping every dlopen
/ dlsym attempt so the failure mode is debuggable.

### Install the Go binding

```bash
go get github.com/julianshen/golibreofficekit/lok
```

Or install one of the bundled CLIs:

```bash
go install github.com/julianshen/golibreofficekit/cmd/lokconv@latest
go install github.com/julianshen/golibreofficekit/cmd/lokmd@latest
```

## Development

Common commands (`Makefile` is the source of truth):

```bash
make build             # go build ./...
make test              # go test -race ./...                 (no LO needed)
make test-integration  # adds -tags=lok_integration; needs LibreOffice + LOK_PATH
make cover             # writes coverage.out and coverage.html
make lint              # go vet + gofmt check
```

Project hand-off / contributor notes:

- [`CONTRIBUTING.md`](./CONTRIBUTING.md) — TDD discipline, branch naming, local workflow
- [`ARCHITECTURE.md`](./ARCHITECTURE.md) — package layout, threading, callbacks, error model
- [`CLAUDE.md`](./CLAUDE.md) — long-form project rules (originally for AI agent contributors)
- [`CHANGELOG.md`](./CHANGELOG.md) — release notes
- [`docs/retros/`](./docs/retros/) — postmortems

## Licence

[Apache License 2.0](./LICENSE). Bundled LibreOfficeKit headers under
`third_party/lok/` are MPL-2.0 (see `third_party/lok/LICENSE` and
[`NOTICE`](./NOTICE)).
