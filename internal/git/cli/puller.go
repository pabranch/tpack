package cli

import (
	"context"
	"fmt"
	"os/exec"
	"strings"

	"github.com/tmuxpack/tpack/internal/git"
)

// Pulls updates for an existing repository using the git CLI.
type Puller struct{}

func NewPuller() *Puller {
	return &Puller{}
}

func (c *Puller) Pull(ctx context.Context, opts git.PullOptions) (string, error) {
	if opts.Branch != "" {
		checkoutCmd := exec.CommandContext(ctx, "git", "checkout", opts.Branch)
		checkoutCmd.Dir = opts.Dir
		checkoutCmd.Env = append(checkoutCmd.Environ(), "GIT_TERMINAL_PROMPT=0")
		if err := checkoutCmd.Run(); err != nil {
			return "", fmt.Errorf("git checkout %s: %w", opts.Branch, err)
		}
	}

	// git pull
	pullCmd := exec.CommandContext(ctx, "git", "pull", "--rebase=false")
	pullCmd.Dir = opts.Dir
	pullCmd.Env = append(pullCmd.Environ(), "GIT_TERMINAL_PROMPT=0")
	out, err := pullCmd.CombinedOutput()
	if err != nil {
		return strings.TrimSpace(string(out)), err
	}

	// Submodules are best-effort: a plugin whose submodule references an
	// unreachable commit (see issue #14) should still update. The failure
	// is surfaced via OnWarning so callers can notify the user.
	subCmd := exec.CommandContext(ctx, "git", "submodule", "update", "--init", "--recursive")
	subCmd.Dir = opts.Dir
	subCmd.Env = append(subCmd.Environ(), "GIT_TERMINAL_PROMPT=0")
	if subOut, subErr := subCmd.CombinedOutput(); subErr != nil && opts.OnWarning != nil {
		opts.OnWarning("submodule update failed: " + strings.TrimSpace(string(subOut)))
	}

	return strings.TrimSpace(string(out)), nil
}
