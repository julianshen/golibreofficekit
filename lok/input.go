//go:build linux || darwin

package lok

import "strings"

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

// String renders a pipe-separated list of the set bits, or "(none)"
// when no bits are set. Order: Left, Right, Middle.
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

// String renders a pipe-separated list of the set bits, or "(none)"
// when no bits are set. Order: Shift, Mod1, Mod2, Mod3.
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
	return strings.Join(parts, "|")
}

// Named key-code constants. A curated subset of
// com::sun::star::awt::Key (IDL: offapi/com/sun/star/awt/Key.idl).
// KeyCodeEnter maps to awt::Key::RETURN. Callers needing keys
// outside this set pass the raw awt::Key int directly.
const (
	KeyCodeEnter     = 1280 // awt::Key::RETURN
	KeyCodeEsc       = 1281 // awt::Key::ESCAPE
	KeyCodeTab       = 1282 // awt::Key::TAB
	KeyCodeBackspace = 1283 // awt::Key::BACKSPACE
	KeyCodeDelete    = 1286 // awt::Key::DELETE
	KeyCodeUp        = 1024 // awt::Key::UP
	KeyCodeDown      = 1025 // awt::Key::DOWN
	KeyCodeLeft      = 1026 // awt::Key::LEFT
	KeyCodeRight     = 1027 // awt::Key::RIGHT
	KeyCodeHome      = 1028 // awt::Key::HOME
	KeyCodeEnd       = 1029 // awt::Key::END
	KeyCodePageUp    = 1030 // awt::Key::PAGEUP
	KeyCodePageDown  = 1031 // awt::Key::PAGEDOWN
)
