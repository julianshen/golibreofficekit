package cli

import (
	"fmt"
	"os"
)

// Die writes "<prefix>: <formatted message>\n" to stderr and exits
// with status 1. Used by CLI mains for runtime-error termination —
// the prefix is the binary name (e.g. "lokconv", "lokmd") so the
// caller can grep for which tool reported.
func Die(prefix, format string, args ...any) {
	fmt.Fprintf(os.Stderr, "%s: %s\n", prefix, fmt.Sprintf(format, args...))
	os.Exit(1)
}
