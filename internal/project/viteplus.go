package project

import (
	"context"
	"fmt"
)

// viteplusInstall is the command the Vite+ docs give to install the global vp
// CLI on macOS and Linux.
const viteplusInstall = "curl -fsSL https://vite.plus | bash"

// checkViteplus checks that the global vp CLI is on PATH and that
// "vp env doctor" passes in dir. A TypeScript component needs both (ADR 0011).
// Without the check, the "vp install" follow-up step would fail after sow had
// written the project. The error says how to install Vite+, or carries the
// doctor's output.
func checkViteplus(ctx context.Context, environ []string, dir string) error {
	if _, err := lookPath(environ, "vp"); err != nil {
		return fmt.Errorf("%w: a TypeScript component needs Vite+. Install it with %q, "+
			"turn on environment management when the installer asks, and open a new shell",
			err, viteplusInstall)
	}
	if err := run(ctx, environ, dir, "vp", "env", "doctor"); err != nil {
		return fmt.Errorf("a TypeScript component needs a working Vite+: "+
			"fix what vp env doctor reports, then try again: %w", err)
	}
	return nil
}
