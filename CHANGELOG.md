# Changelog

All notable changes to this project will be documented in this file.

The format is based on [Keep a Changelog](https://keepachangelog.com/en/1.1.0/),
and this project adheres to [Semantic Versioning](https://semver.org/spec/v2.0.0.html).

## [Unreleased]

## [1.0.0] — 2026-04-28

First tagged release. The public API in [`lok`](./lok) is now considered stable
under SemVer; breaking changes will bump the major version.

### Added — `lok` package

- `Office` lifecycle: `New`, `Close`, `SetAuthor`, `TrimMemory`, `DumpState`,
  `SetDocumentPassword`, `AddListener`, `DroppedEvents`, `PanickedListeners`,
  `WithUserProfile` option.
- `Document`: `Load` / `LoadFromReader` (with `WithPassword`, `WithReadOnly`,
  `WithLanguage`, `WithMacroSecurity`, `WithBatchMode`, `WithRepair`,
  `WithFilterOptions`), `Type`, `Save`, `SaveAs`, `Close`, `AddListener`,
  `DroppedEvents`, `PanickedListeners`.
- Views: `CreateView`, `CreateViewWithOptions`, `DestroyView`, `SetView`,
  `View`, `Views`, `SetViewLanguage`, `SetViewReadOnly`,
  `SetAccessibilityState`, `SetViewTimezone`.
- Parts & sizing: `Parts`, `Part`, `SetPart`, `SetPartMode`, `PartName`,
  `PartHash`, `PartInfo`, `DocumentSize`, `PartPageRectangles`,
  `SetOutlineState`.
- Rendering: `InitializeForRendering`, `SetClientZoom`,
  `SetClientVisibleArea`, `PaintTile` / `PaintTileRaw`, `PaintPartTile` /
  `PaintPartTileRaw`, `RenderImage` / `RenderPNG`, `RenderPage` /
  `RenderPagePNG`, `RenderSearchResult` / `RenderSearchResultRaw`,
  `RenderShapeSelection`, `RenderFont`.
- Input events: `PostKeyEvent`, `PostMouseEvent`, `PostUnoCommand` plus
  typed helpers (`Bold`, `Italic`, `Underline`, `Undo`, `Redo`, `Copy`,
  `Cut`, `Paste`, `SelectAll`, …).
- Selection & clipboard: `GetTextSelection`, `GetSelectionType`,
  `GetSelectionTypeAndText`, `SetTextSelection`, `ResetSelection`,
  `SetGraphicSelection`, `SetBlockedCommandList`, `GetClipboard`,
  `SetClipboard`, `Paste`, `SelectPart`, `MoveSelectedParts`.
- Window events: `PostWindowKeyEvent`, `PostWindowMouseEvent`,
  `PostWindowGestureEvent`, `PostWindowExtTextInputEvent`, `ResizeWindow`,
  `PaintWindow`, `PaintWindowDPI`, `PaintWindowForView`.
- Commands & forms: `GetCommandValues`, `CompleteFunction`,
  `SendDialogEvent`, `SendContentControlEvent`, `SendFormFieldEvent`.
- Advanced: `RunMacro`, `Sign`, `InsertCertificate`, `AddCertificate`,
  `SignatureState`, `FilterTypes`, `OptionalFeatures`.
- Errors: `ErrInstallPathRequired`, `ErrAlreadyInitialised`, `ErrClosed`,
  `ErrPathRequired`, `ErrInvalidOption`, `ErrViewCreateFailed`,
  `ErrUnsupported`, `ErrMacroFailed`, `ErrSignFailed`, `ErrPasteFailed`,
  `ErrNoValue`, `ErrClipboardFailed`, plus `*LOKError{Op, Detail}` carrying
  LibreOffice's own `getError()` string.
- Listener model with bounded per-listener buffer, dropped-event counter,
  and panic-recovered dispatcher with a panicked-listener counter.

### Added — CLI tools

- `cmd/lokconv`: convert documents (`.docx`, `.odt`, `.xlsx`, `.ods`,
  `.pptx`, `.odp`, `.odg`, …) to PDF or PNG. Output format inferred from
  the `-out` extension.
- `cmd/lokmd`: bidirectional Markdown ↔ DOCX/PPTX, with a Marp-compatible
  slide pipeline (`---` separators, YAML front matter, `# ` headings).

### Changed

- All `lok` methods touching optional LOK vtable slots return `error` so a
  stripped LibreOffice build surfaces `ErrUnsupported` instead of silently
  no-opping. (Previous internal builds had void signatures on
  `PostKeyEvent`, `PostMouseEvent`, `SetView`, `SetPart`, `SetPartMode`,
  `SetOutlineState`, `InitializeForRendering`, `SetClientZoom`,
  `SetClientVisibleArea`, `PostUnoCommand`, `SetViewLanguage`,
  `SetViewReadOnly`, `SetAccessibilityState`, `SetViewTimezone`,
  `DestroyView`.)
- `Office.Load` / `Office.LoadDocumentWithOptions` / `Document.SaveAs` /
  `Document.Save` now consult LibreOffice's own `getError()` and report it
  in `*LOKError.Detail` so users see the real reason for a failure
  ("password required", "filter rejected file") instead of a generic
  "documentLoad returned NULL".

### Notes

- Linux and macOS only; Windows is not yet supported.
- Requires LibreOffice 24.8 or newer at runtime for integration paths.
- Module is on Go 1.23.

[Unreleased]: https://github.com/julianshen/golibreofficekit/compare/v1.0.0...HEAD
[1.0.0]: https://github.com/julianshen/golibreofficekit/releases/tag/v1.0.0
