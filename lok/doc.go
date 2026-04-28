// Package lok is the public, idiomatic Go binding for LibreOfficeKit.
//
// LibreOfficeKit (LOK) is the C ABI exposed by LibreOffice for loading,
// rendering, editing, and saving documents in-process — Writer (.odt /
// .docx), Calc (.ods / .xlsx), Impress (.odp / .pptx), and Draw (.odg).
// This package wraps that ABI through cgo so Go programs can drive
// LibreOffice without spawning a subprocess.
//
// # Quickstart
//
// Open an office, load a document, save it as a different format,
// close everything down:
//
//	office, err := lok.New("/usr/lib64/libreoffice/program")
//	if err != nil {
//	    return err
//	}
//	defer office.Close()
//
//	doc, err := office.Load("report.docx")
//	if err != nil {
//	    return err
//	}
//	defer doc.Close()
//
//	if err := doc.SaveAs("report.pdf", "pdf", ""); err != nil {
//	    return err
//	}
//
// Render the first page of a presentation to a PNG file:
//
//	doc, err := office.Load("deck.pptx")
//	if err != nil { return err }
//	defer doc.Close()
//
//	if err := doc.InitializeForRendering(""); err != nil {
//	    return err
//	}
//	png, err := doc.RenderPagePNG(0, 1.0) // page index, DPI scale
//	if err != nil { return err }
//	_ = os.WriteFile("slide-1.png", png, 0o644)
//
// The cmd/lokconv and cmd/lokmd subcommands are end-to-end CLI examples
// built on this package.
//
// # Concepts
//
// [New] returns a process-singleton *Office. LibreOffice's lok_init may
// only be called once per process; calling [New] a second time while
// the first is open returns [ErrAlreadyInitialised]. The matching
// teardown is [Office.Close].
//
// Each loaded document is an *Document. A process may hold many
// documents at once (each with its own [Document.Close]) but every
// document method serialises on the parent Office's mutex — LOK is not
// thread-safe internally.
//
// LO callbacks (selection changes, invalidations, save status, …)
// arrive via [Office.AddListener] / [Document.AddListener]. The
// dispatcher drops events that overflow the per-listener buffer and
// recovers from listener panics; [Office.DroppedEvents],
// [Document.DroppedEvents], [Office.PanickedListeners], and
// [Document.PanickedListeners] expose the counters so a misbehaving
// consumer is observable rather than silent.
//
// # Install paths
//
// [New] needs the absolute path of LibreOffice's program/ directory.
// Common defaults:
//
//   - Linux (Fedora):        /usr/lib64/libreoffice/program
//   - Linux (Debian/Ubuntu): /usr/lib/libreoffice/program
//   - macOS:                 inside the LibreOffice.app bundle, e.g.
//     /Applications/LibreOffice.app/Contents/Frameworks
//
// If the path is wrong [New] returns an error wrapping the underlying
// dlopen failure with all candidate library names tried.
//
// # Errors
//
// All exported functions return error rather than panicking. Sentinels
// usable with errors.Is include [ErrUnsupported] when a LOK build
// lacks a vtable slot, [ErrClosed] when the *Office has been Closed,
// and [ErrInvalidOption] for malformed load options. LOK's own error
// strings (returned by getError() on the C side) are surfaced through
// [LOKError] so callers see the real reason for a failure (password
// required, filter rejected the file, …) instead of a generic
// "documentLoad returned NULL".
//
// # Threading
//
// This package serialises every LOK call through a per-Office mutex,
// so concurrent goroutines may share an *Office and *Document safely.
// Long-running renders or saves block other callers. Listener
// callbacks run on a dedicated dispatcher goroutine owned by the
// *Office; do not call back into the same document from inside a
// listener — that re-enters the LOK mutex and deadlocks.
//
// # Stability
//
// The module is pre-1.0; method signatures may still change. Methods
// that touch optional LOK vtable slots return an error so a stripped
// LibreOffice build surfaces [ErrUnsupported] instead of silently
// no-opping. See the CHANGELOG for any source-incompatible change
// between versions.
package lok
