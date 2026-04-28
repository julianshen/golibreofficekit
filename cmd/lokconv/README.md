# lokconv

A small CLI that uses the `golibreofficekit` binding to convert any
LibreOffice-readable document (Writer, Calc, Impress, Draw, plus the
formats LO can import — `.docx`, `.xlsx`, `.pptx`, etc.) to either
PDF or PNG.

## Install

```
go install github.com/julianshen/golibreofficekit/cmd/lokconv@latest
```

The binary is statically-linked Go but loads LibreOffice via `dlopen`
at runtime — you need a real LO install on the machine that runs it.

## Usage

```
lokconv -in PATH -out PATH [-page N] [-dpi SCALE] [-lo-path DIR]
```

Output format is inferred from the `-out` extension:

- `.pdf` → goes through LO's PDF export filter (whole document)
- `.png` → renders one page via LO's tile pipeline

Flags:

| Flag      | Default                                   | Notes                                            |
|-----------|-------------------------------------------|--------------------------------------------------|
| `-in`     | _(required)_                              | input document path                              |
| `-out`    | _(required)_                              | output `.pdf` or `.png` path                     |
| `-page`   | `0`                                       | page index for PNG (sheet/slide/page; 0-based)   |
| `-dpi`    | `1.0`                                     | DPI scale (1.0 = 96 DPI, 2.0 = 192 DPI, etc.)    |
| `-lo-path`| `$LOK_PATH` then auto-detect              | LibreOffice install path                         |

Auto-detect tries (first existing wins):

```
/usr/lib/libreoffice/program        # Debian/Ubuntu (apt)
/usr/lib64/libreoffice/program      # Fedora/RHEL
/Applications/LibreOffice.app/Contents/Frameworks  # macOS .app bundle
```

## Examples

```sh
# Writer → PDF
lokconv -in report.docx -out report.pdf

# A specific Impress slide as a PNG at 1.5× DPI
lokconv -in deck.pptx -out slide-2.png -page 1 -dpi 1.5

# Calc sheet 0 as a PNG, explicit LO path
lokconv -in sales.xlsx -out sales.png -lo-path /opt/libreoffice/program
```

## Page semantics

The `-page` flag follows the binding's `Document.RenderPage` semantics:

- **Calc** — sheet index (0 = first sheet)
- **Impress** — slide index
- **Draw** — page index
- **Writer** — page within the current part (Writer typically has
  one part), bounded by LO's `getPartPageRectangles`. For
  single-page output of the whole vertical layout use the binding's
  `Document.RenderPNG` directly; `lokconv` does not expose that
  variant today.

## Limitations

- One-shot per invocation. LibreOffice's `lok_init` cannot be
  re-invoked within a process, so each `lokconv` run starts and
  tears down a fresh LO. For batch jobs, prefer scripting the
  binding directly.
- PNG output is one page per call — for multi-page exports loop
  over `-page` indices.
- `-out png` always uses LO's tile-paint pipeline, not LO's
  `png_Export` filter (which only Draw/Impress expose). The two
  routes can differ in pixel-perfect output for the same source.
