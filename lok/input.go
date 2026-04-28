//go:build linux || darwin

package lok

import (
	"fmt"
	"math"
	"strings"
)

// KeyEventType mirrors LOK_KEYEVENT_*.
type KeyEventType int

const (
	KeyEventInput KeyEventType = 0 // LOK_KEYEVENT_KEYINPUT
	KeyEventUp    KeyEventType = 1 // LOK_KEYEVENT_KEYUP
)

// MouseEventType mirrors LOK_MOUSEEVENT_*.
type MouseEventType int

const (
	MouseButtonDown MouseEventType = 0 // LOK_MOUSEEVENT_MOUSEBUTTONDOWN
	MouseButtonUp   MouseEventType = 1 // LOK_MOUSEEVENT_MOUSEBUTTONUP
	MouseMove       MouseEventType = 2 // LOK_MOUSEEVENT_MOUSEMOVE
)

// MouseButton is a UNO awt::MouseButton bitset.
type MouseButton uint16

const (
	MouseLeft   MouseButton = 1
	MouseRight  MouseButton = 2
	MouseMiddle MouseButton = 4
)

// Has reports whether all bits in other are set in b. b.Has(0) is
// true by definition.
func (b MouseButton) Has(other MouseButton) bool {
	return b&other == other
}

// allMouseButtons masks the named bits. Bits outside this mask are
// rendered as a hex fallback by String.
const allMouseButtons = MouseLeft | MouseRight | MouseMiddle

// String renders a pipe-separated list of the set bits, or "(none)"
// when no bits are set. Named bits come first in the fixed order
// Left, Right, Middle; any unknown remainder is appended as "0xN"
// so values outside the named set round-trip losslessly.
func (b MouseButton) String() string {
	if b == 0 {
		return "(none)"
	}
	var parts []string
	if b.Has(MouseLeft) {
		parts = append(parts, "MouseLeft")
	}
	if b.Has(MouseRight) {
		parts = append(parts, "MouseRight")
	}
	if b.Has(MouseMiddle) {
		parts = append(parts, "MouseMiddle")
	}
	if rest := b &^ allMouseButtons; rest != 0 {
		parts = append(parts, fmt.Sprintf("0x%x", uint16(rest)))
	}
	return strings.Join(parts, "|")
}

// Modifier is a UNO awt::KeyModifier bitset.
type Modifier uint16

const (
	ModShift Modifier = 1
	ModMod1  Modifier = 2 // Ctrl on Linux/Windows, Cmd on macOS
	ModMod2  Modifier = 4 // Alt / Option
	ModMod3  Modifier = 8
)

// Has reports whether all bits in other are set in m. m.Has(0) is
// true by definition.
func (m Modifier) Has(other Modifier) bool {
	return m&other == other
}

// allModifiers masks the named bits. Bits outside this mask are
// rendered as a hex fallback by String.
const allModifiers = ModShift | ModMod1 | ModMod2 | ModMod3

// String renders a pipe-separated list of the set bits, or "(none)"
// when no bits are set. Named bits come first in the fixed order
// Shift, Mod1, Mod2, Mod3; any unknown remainder is appended as
// "0xN" so values outside the named set round-trip losslessly.
func (m Modifier) String() string {
	if m == 0 {
		return "(none)"
	}
	var parts []string
	if m.Has(ModShift) {
		parts = append(parts, "ModShift")
	}
	if m.Has(ModMod1) {
		parts = append(parts, "ModMod1")
	}
	if m.Has(ModMod2) {
		parts = append(parts, "ModMod2")
	}
	if m.Has(ModMod3) {
		parts = append(parts, "ModMod3")
	}
	if rest := m &^ allModifiers; rest != 0 {
		parts = append(parts, fmt.Sprintf("0x%x", uint16(rest)))
	}
	return strings.Join(parts, "|")
}

// Named key-code constants. A curated subset of
// com::sun::star::awt::Key (IDL: offapi/com/sun/star/awt/Key.idl).
// KeyCodeEnter maps to awt::Key::RETURN. Callers needing keys
// outside this set pass the raw awt::Key int directly. Typed as
// int so the constant flows into PostKeyEvent's keyCode parameter
// without ambiguity about the underlying numeric type.
const (
	KeyCodeEnter     int = 1280 // awt::Key::RETURN
	KeyCodeEsc       int = 1281 // awt::Key::ESCAPE
	KeyCodeTab       int = 1282 // awt::Key::TAB
	KeyCodeBackspace int = 1283 // awt::Key::BACKSPACE
	KeyCodeDelete    int = 1286 // awt::Key::DELETE
	KeyCodeUp        int = 1024 // awt::Key::UP
	KeyCodeDown      int = 1025 // awt::Key::DOWN
	KeyCodeLeft      int = 1026 // awt::Key::LEFT
	KeyCodeRight     int = 1027 // awt::Key::RIGHT
	KeyCodeHome      int = 1028 // awt::Key::HOME
	KeyCodeEnd       int = 1029 // awt::Key::END
	KeyCodePageUp    int = 1030 // awt::Key::PAGEUP
	KeyCodePageDown  int = 1031 // awt::Key::PAGEDOWN
)

// PostKeyEvent posts a keyboard event to the currently active view.
// charCode is a Unicode code point (0 for non-printables); keyCode
// is an awt::Key value (0 for plain characters). The caller is
// responsible for pairing KeyEventInput with a matching KeyEventUp —
// LOK does not synthesize a release.
//
// LOK exposes no synchronous result for input events; mutations become
// observable only through a registered document callback. Values of
// charCode or keyCode outside int32 return *LOKError{Op:"PostKeyEvent"}
// without invoking LOK. Returns ErrUnsupported when the LOK build does
// not expose postKeyEvent (vtable slot NULL) — silent no-op was
// indistinguishable from success on stripped LO builds.
func (d *Document) PostKeyEvent(typ KeyEventType, charCode, keyCode int) error {
	if err := requireInt32Key("PostKeyEvent", charCode, keyCode); err != nil {
		return err
	}
	unlock, err := d.guard()
	if err != nil {
		return err
	}
	defer unlock()
	return d.office.be.DocumentPostKeyEvent(d.h, int(typ), charCode, keyCode)
}

// PostMouseEvent posts a mouse event at twip coordinates (x, y).
// count is the click count (1 for single, 2 for double, etc.); for
// MouseMove the binding passes count through unchanged, with 0 the
// conventional value. buttons and mods are OR-ed bitsets.
//
// LOK exposes no synchronous result for input events; mutations become
// observable only through a registered document callback. Values of
// x or y outside int32 return *LOKError{Op:"PostMouseEvent"} without
// invoking LOK. Returns ErrUnsupported when the LOK build does not
// expose postMouseEvent (vtable slot NULL).
func (d *Document) PostMouseEvent(typ MouseEventType, x, y int64, count int, buttons MouseButton, mods Modifier) error {
	if err := requireInt32XY("PostMouseEvent", x, y); err != nil {
		return err
	}
	unlock, err := d.guard()
	if err != nil {
		return err
	}
	defer unlock()
	return d.office.be.DocumentPostMouseEvent(d.h, int(typ), int(x), int(y),
		count, int(buttons), int(mods))
}

// PostUnoCommand dispatches a .uno:* command to the active view.
// argsJSON is LOK's raw JSON args string (may be empty).
// notifyWhenFinished requests a LOK_CALLBACK_UNO_COMMAND_RESULT;
// the flag is forwarded verbatim but results, and any dispatcher-side
// failures, are observable only through a registered document
// callback — LOK exposes no synchronous error channel for UNO
// dispatch. Returns ErrUnsupported when the LOK build does not
// expose postUnoCommand (vtable slot NULL).
func (d *Document) PostUnoCommand(cmd, argsJSON string, notifyWhenFinished bool) error {
	unlock, err := d.guard()
	if err != nil {
		return err
	}
	defer unlock()
	return d.office.be.DocumentPostUnoCommand(d.h, cmd, argsJSON, notifyWhenFinished)
}

// requireInt32XY returns *LOKError if x or y exceeds int32 range.
// LOK's postMouseEvent takes C int (32-bit on LP64).
func requireInt32XY(op string, x, y int64) error {
	if x > math.MaxInt32 || x < math.MinInt32 ||
		y > math.MaxInt32 || y < math.MinInt32 {
		return &LOKError{Op: op, Detail: fmt.Sprintf("coord out of int32 range: x=%d, y=%d", x, y)}
	}
	return nil
}

// requireInt32 returns *LOKError if v exceeds int32 range. name labels
// the offending parameter in the error message.
func requireInt32(op, name string, v int64) error {
	if v > math.MaxInt32 || v < math.MinInt32 {
		return &LOKError{Op: op, Detail: fmt.Sprintf("%s out of int32 range: %s=%d", name, name, v)}
	}
	return nil
}

// requireInt32Key returns *LOKError if charCode or keyCode exceeds
// int32 range. LOK's postKeyEvent takes C int (32-bit on LP64);
// without this guard values outside that range would silently
// truncate at the cgo boundary.
func requireInt32Key(op string, charCode, keyCode int) error {
	if charCode > math.MaxInt32 || charCode < math.MinInt32 ||
		keyCode > math.MaxInt32 || keyCode < math.MinInt32 {
		return &LOKError{Op: op,
			Detail: fmt.Sprintf("charCode/keyCode out of int32 range: charCode=%d, keyCode=%d",
				charCode, keyCode)}
	}
	return nil
}
