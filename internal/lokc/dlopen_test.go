//go:build linux || darwin

package lokc

import (
	"errors"
	"strings"
	"testing"
)

func TestDLError_ErrorMessage(t *testing.T) {
	err := &DLError{Op: OpDLOpen, Target: "/no/such.so", Detail: "not found"}
	got := err.Error()
	for _, want := range []string{"dlopen", "/no/such.so", "not found"} {
		if !strings.Contains(got, want) {
			t.Errorf("Error()=%q, missing %q", got, want)
		}
	}
}

func TestDLOpen_MissingFileReturnsError(t *testing.T) {
	_, err := dlOpen("/this/path/does/not/exist.so")
	if err == nil {
		t.Fatal("expected error for missing .so, got nil")
	}
	var dlerr *DLError
	if !errors.As(err, &dlerr) {
		t.Fatalf("expected *DLError, got %T (%v)", err, err)
	}
	if dlerr.Op != OpDLOpen {
		t.Errorf("Op: want %q, got %q", OpDLOpen, dlerr.Op)
	}
}

func TestDLOpen_EmptyPathOpensMainProgram(t *testing.T) {
	// dlOpen("") translates to dlopen(NULL), which returns a handle
	// to the main program on both Linux and macOS; libc symbols are
	// resolvable through it.
	handle, err := dlOpen("")
	if err != nil {
		t.Fatalf("dlOpen(\"\"): %v", err)
	}
	if handle == nil {
		t.Fatal("handle is nil")
	}
}

func TestDLSym_FindsMalloc(t *testing.T) {
	handle, err := dlOpen("")
	if err != nil {
		t.Skip("cannot open self:", err)
	}
	p, err := dlSym(handle, "malloc")
	if err != nil {
		t.Fatalf("dlsym malloc: %v", err)
	}
	if p == nil {
		t.Fatal("malloc resolved to nil")
	}
}

func TestDLSym_NilHandleErrors(t *testing.T) {
	_, err := dlSym(nil, "malloc")
	if err == nil {
		t.Fatal("expected error for nil handle")
	}
	var dlerr *DLError
	if !errors.As(err, &dlerr) || dlerr.Op != OpDLSym {
		t.Errorf("want *DLError Op=%q, got %T %v", OpDLSym, err, err)
	}
}

func TestDLSym_MissingSymbolErrors(t *testing.T) {
	handle, err := dlOpen("")
	if err != nil {
		t.Skip("cannot open self:", err)
	}
	_, err = dlSym(handle, "definitely_not_a_real_symbol_zzz")
	if err == nil {
		t.Fatal("expected error for missing symbol")
	}
}

func TestLastDLError_NoErrorReturnsPlaceholder(t *testing.T) {
	// After a successful dlopen/dlsym, dlerror() returns NULL.
	// Calling lastDLError when there is no pending error must
	// return the "(no dlerror)" placeholder, not an empty string.
	handle, err := dlOpen("")
	if err != nil {
		t.Skip("cannot open main program:", err)
	}
	if _, err = dlSym(handle, "malloc"); err != nil {
		t.Skip("cannot find malloc:", err)
	}
	// dlerror() is now NULL — lastDLError should hit the cs==nil branch.
	got := lastDLError()
	if got != "(no dlerror)" {
		t.Errorf("lastDLError()=%q, want %q", got, "(no dlerror)")
	}
}
