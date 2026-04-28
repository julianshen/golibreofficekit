# lokmd

Convert between Markdown and Office formats — `.docx` and `.pptx` —
using the `golibreofficekit` binding plus a small Marp-compatible
markdown→presentation pipeline.

## Install

```
go install github.com/julianshen/golibreofficekit/cmd/lokmd@latest
```

The binary is statically-linked Go but loads LibreOffice via `dlopen`
at runtime — you need a real LO install (24.x or newer) on the
machine that runs it.

## Usage

```
lokmd -in PATH -out PATH [-lo-path DIR]
```

Direction is inferred from the file extensions:

| Input → Output | How it works |
|---|---|
| `.md`/`.markdown` → `.docx` | LO's Markdown import filter + Word export |
| `.docx` → `.md`/`.markdown`  | LO's Markdown export filter |
| `.md`/`.markdown` → `.pptx` | Markdown parsed Marp-style, materialised as a flat-XML `.fodp`, then LO saves as `.pptx` |
| `.pptx` → `.md`/`.markdown` | LO saves the deck as flat-XML `.fodp`; lokmd walks the XML for slide titles and body paragraphs |

LO's built-in Markdown filter is **Writer-only**, so the `.pptx`
directions go through a Marpit-style markdown convention:

- `---` on its own line separates slides
- An optional YAML front-matter block (`---` … `---` at the very top)
  is stripped before parsing — the directives inside aren't honoured
  yet but the source stays Marp-compatible
- The first `# ` heading per slide becomes the slide title; the rest
  of the slide is body text. Bullet lines (`- ` or `* `) survive the
  round-trip as separate paragraphs

## Example

A small Marp-style deck:

```markdown
---
marp: true
theme: default
---

# Hello

First slide body.

---

# World

- bullet one
- bullet two
```

Convert it both ways:

```
lokmd -in deck.md   -out deck.pptx
lokmd -in deck.pptx -out deck.md   # round-trips back to ~the same markdown
```

## Flags

| Flag      | Default                       | Notes                                         |
|-----------|-------------------------------|-----------------------------------------------|
| `-in`     | _(required)_                  | input document                                |
| `-out`    | _(required)_                  | output document; format inferred from ext     |
| `-lo-path`| `$LOK_PATH` then auto-detect  | LibreOffice install path                      |

Auto-detect tries (first existing wins):

```
/usr/lib/libreoffice/program        # Debian/Ubuntu (apt)
/usr/lib64/libreoffice/program      # Fedora/RHEL
/Applications/LibreOffice.app/Contents/Frameworks  # macOS .app bundle
```

## Limitations

- **One-shot per invocation.** `lok_init` cannot be re-invoked in a
  process, so each `lokmd` run starts and tears down a fresh LO. For
  bulk jobs, prefer scripting the binding directly.
- **Markdown rendering is intentionally minimal.** Bold/italic, code
  blocks, images, tables, and Marp's slide-level directives
  (`<!-- _class: lead -->` etc.) are not honoured by the
  `.md → .pptx` pipeline. The conversion is "preserve titles and
  paragraph text"; visual fidelity is LO's job after import.
- **Only md ↔ docx and md ↔ pptx are supported.** Going `.docx ↔ .pptx`
  is rejected with a clear error — LO can't cross document kinds and
  silently dropping content (slides → paragraphs or vice versa) would
  be the wrong default.
- **Front-matter directives are ignored.** A `marp: true` block is
  stripped so `lokmd` can read Marp source files unmodified, but
  theme/class directives are not applied.
