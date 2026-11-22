// Package gitcfg applies a git identity to a repository.
package gitcfg

import (
	"bytes"
	"fmt"
	"os"
	"os/exec"
	"path/filepath"
	"strings"
)

// Identity is the set of values written into a repository's local config.
type Identity struct {
	Name  string
	Email string
	Key   string
}

// Result describes what happened during Apply.
type Result struct {
	// Repo is the absolute path to the repository root.
	Repo string
	// Initialized reports whether `git init` had to be run.
	Initialized bool
	// SSHCommand is the value written to core.sshCommand.
	SSHCommand string
}

// Apply writes user.name, user.email and core.sshCommand into the local git
// config of the repository containing dir. If dir is not inside a repository,
// it is initialized with `git init` first.
func Apply(dir string, id Identity) (Result, error) {
	if _, err := exec.LookPath("git"); err != nil {
		return Result{}, fmt.Errorf("git не найден в PATH — установите git")
	}

	var res Result
	root, err := git(dir, "rev-parse", "--show-toplevel")
	if err != nil {
		if _, initErr := git(dir, "init", "-q"); initErr != nil {
			return Result{}, initErr
		}
		res.Initialized = true
		root, err = git(dir, "rev-parse", "--show-toplevel")
		if err != nil {
			// A very old git or a weird setup: fall back to the working
			// directory, the local config still lives in .git/config.
			root = dir
		}
	}
	res.Repo = strings.TrimSpace(root)
	res.SSHCommand = SSHCommand(id.Key)

	settings := [][2]string{
		{"user.name", id.Name},
		{"user.email", id.Email},
		{"core.sshCommand", res.SSHCommand},
	}
	for _, kv := range settings {
		if _, err := git(dir, "config", "--local", kv[0], kv[1]); err != nil {
			return res, err
		}
	}
	return res, nil
}

// SSHCommand builds the core.sshCommand value for a key name or path.
func SSHCommand(key string) string {
	return "ssh -i " + shellQuote(KeyPath(key)) + " -o IdentitiesOnly=yes"
}

// KeyPath resolves a configured key to a filesystem path:
// absolute paths are used as-is, "~/..." is expanded to the home directory,
// and any relative path (including a bare file name) is resolved under ~/.ssh.
func KeyPath(key string) string {
	key = strings.TrimSpace(key)
	home, _ := os.UserHomeDir()

	if strings.HasPrefix(key, "~/") && home != "" {
		return filepath.Join(home, key[2:])
	}
	if filepath.IsAbs(key) {
		return key
	}
	if home != "" {
		return filepath.Join(home, ".ssh", key)
	}
	return key
}

// shellQuote quotes s for git's shell-based execution of core.sshCommand.
func shellQuote(s string) string {
	plain := s != ""
	for _, r := range s {
		switch r {
		case ' ', '\t', '\n', '\'', '"', '\\', '$', '`', '!', '&', '|', ';',
			'<', '>', '(', ')', '*', '?', '[', ']', '{', '}', '#', '~':
			plain = false
		}
		if !plain {
			break
		}
	}
	if plain {
		return s
	}
	return "'" + strings.ReplaceAll(s, "'", `'\''`) + "'"
}

func git(dir string, args ...string) (string, error) {
	cmd := exec.Command("git", args...)
	cmd.Dir = dir
	var stdout, stderr bytes.Buffer
	cmd.Stdout = &stdout
	cmd.Stderr = &stderr
	if err := cmd.Run(); err != nil {
		msg := strings.TrimSpace(stderr.String())
		if msg == "" {
			msg = err.Error()
		}
		return stdout.String(), fmt.Errorf("git %s: %s", strings.Join(args, " "), msg)
	}
	return stdout.String(), nil
}
