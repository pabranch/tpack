package cli_test

import (
	"context"
	"os"
	"os/exec"
	"path/filepath"
	"strings"
	"testing"

	"github.com/tmuxpack/tpack/internal/git"
	gitcli "github.com/tmuxpack/tpack/internal/git/cli"
)

func TestCloner_CloneSuccess(t *testing.T) {
	if testing.Short() {
		t.Skip("skipping git CLI test in short mode")
	}

	bare := initBareRepo(t)
	dst := filepath.Join(t.TempDir(), "cloned")

	cloner := gitcli.NewCloner()
	err := cloner.Clone(context.Background(), git.CloneOptions{
		URL: bare,
		Dir: dst,
	})
	if err != nil {
		t.Fatalf("Clone returned error: %v", err)
	}

	// The cloned directory must exist and contain the file from the initial commit.
	if _, err := os.Stat(filepath.Join(dst, "README")); err != nil {
		t.Fatalf("expected README in cloned repo: %v", err)
	}
}

func TestCloner_CloneWithBranch(t *testing.T) {
	if testing.Short() {
		t.Skip("skipping git CLI test in short mode")
	}

	bare := initBareRepo(t)

	// Create a feature branch in the bare repo via a temporary working copy.
	work := cloneLocal(t, bare)
	runGit(t, work, "checkout", "-b", "feature")
	writeFile(t, filepath.Join(work, "feature.txt"), "on feature branch")
	runGit(t, work, "add", ".")
	runGit(t, work, "commit", "-m", "feature commit")
	runGit(t, work, "push", "origin", "feature")

	// Clone specifying the feature branch.
	dst := filepath.Join(t.TempDir(), "cloned-branch")
	cloner := gitcli.NewCloner()
	err := cloner.Clone(context.Background(), git.CloneOptions{
		URL:    bare,
		Dir:    dst,
		Branch: "feature",
	})
	if err != nil {
		t.Fatalf("Clone with branch returned error: %v", err)
	}

	// feature.txt should be present because we cloned the feature branch.
	if _, err := os.Stat(filepath.Join(dst, "feature.txt")); err != nil {
		t.Fatalf("expected feature.txt in cloned repo: %v", err)
	}
}

func TestCloner_CloneInvalidURL(t *testing.T) {
	if testing.Short() {
		t.Skip("skipping git CLI test in short mode")
	}

	dst := filepath.Join(t.TempDir(), "bad-clone")
	cloner := gitcli.NewCloner()
	err := cloner.Clone(context.Background(), git.CloneOptions{
		URL: "/nonexistent/path/to/repo.git",
		Dir: dst,
	})
	if err == nil {
		t.Fatal("expected error when cloning invalid URL")
	}
}

// TestCloner_CloneSubmoduleWithUnreachableCommit reproduces issue #14
// (https://github.com/tmuxpack/tpack/issues/14): cloning a plugin whose
// submodule references a commit unreachable from any ref on the submodule
// remote fails with "fatal: remote error: upload-pack: not our ref ...".
//
// We mirror that condition locally by giving a submodule's gitlink a SHA
// that the bare submodule repo cannot serve, then invoking the production
// Cloner (which uses --recursive). When the bug is present the test fails;
// once tpack handles broken submodules gracefully it should pass.
func TestCloner_CloneSubmoduleWithUnreachableCommit(t *testing.T) {
	if testing.Short() {
		t.Skip("skipping git CLI test in short mode")
	}

	// Default git config blocks file:// URLs for submodules (CVE-2022-39253).
	// The real bug uses https URLs, so allow file:// in the test environment
	// just to make the local reproduction possible.
	t.Setenv("GIT_CONFIG_COUNT", "1")
	t.Setenv("GIT_CONFIG_KEY_0", "protocol.file.allow")
	t.Setenv("GIT_CONFIG_VALUE_0", "always")

	subBare := initBareRepo(t)

	// Create a commit on top of the submodule HEAD without pushing it. Its
	// SHA exists nowhere on the bare remote, so upload-pack will refuse to
	// serve it — the same situation as tmux-powerkit's wiki submodule.
	subWork := cloneLocal(t, subBare)
	writeFile(t, filepath.Join(subWork, "extra.txt"), "extra content")
	runGit(t, subWork, "add", ".")
	runGit(t, subWork, "commit", "-m", "extra commit")
	revOut, err := exec.CommandContext(context.Background(),
		"git", "-C", subWork, "rev-parse", "HEAD").Output()
	if err != nil {
		t.Fatalf("rev-parse HEAD failed: %v", err)
	}
	unreachable := strings.TrimSpace(string(revOut))

	// Build a parent repo whose `wiki` submodule gitlink points at the
	// unreachable SHA.
	parentBare := filepath.Join(t.TempDir(), "parent.git")
	runGit(t, "", "init", "--bare", parentBare)

	parentWork := filepath.Join(t.TempDir(), "parent-work")
	runGit(t, "", "clone", parentBare, parentWork)
	runGit(t, parentWork, "config", "user.email", "test@test.com")
	runGit(t, parentWork, "config", "user.name", "Test")

	writeFile(t, filepath.Join(parentWork, "README"), "parent")
	runGit(t, parentWork, "add", ".")
	runGit(t, parentWork, "commit", "-m", "initial commit")

	// Register the submodule normally so .gitmodules is written, then
	// rewrite the gitlink to the unreachable SHA before committing.
	runGit(t, parentWork, "submodule", "add", subBare, "wiki")
	runGit(t, parentWork, "update-index", "--add",
		"--cacheinfo", "160000,"+unreachable+",wiki")
	runGit(t, parentWork, "commit", "-m", "point wiki at unreachable commit")
	runGit(t, parentWork, "push", "origin", "HEAD")

	dst := filepath.Join(t.TempDir(), "cloned-parent")
	var warnings []string
	cloner := gitcli.NewCloner()
	err = cloner.Clone(context.Background(), git.CloneOptions{
		URL: parentBare,
		Dir: dst,
		OnWarning: func(msg string) {
			warnings = append(warnings, msg)
		},
	})
	if err != nil {
		t.Fatalf("issue #14: clone failed because submodule references an unreachable commit: %v", err)
	}
	if len(warnings) == 0 {
		t.Fatal("expected an OnWarning call describing the failed submodule update")
	}
	if !strings.Contains(warnings[0], "submodule") {
		t.Errorf("warning should mention submodule failure, got %q", warnings[0])
	}
}

func TestCloner_CloneCancelledContext(t *testing.T) {
	if testing.Short() {
		t.Skip("skipping git CLI test in short mode")
	}

	bare := initBareRepo(t)
	dst := filepath.Join(t.TempDir(), "canceled-clone")

	ctx, cancel := context.WithCancel(context.Background())
	cancel() // cancel immediately

	cloner := gitcli.NewCloner()
	err := cloner.Clone(ctx, git.CloneOptions{
		URL: bare,
		Dir: dst,
	})
	if err == nil {
		t.Fatal("expected error when context is canceled")
	}
}
