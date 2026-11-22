package gitcfg

import (
	"path/filepath"
	"testing"
)

func TestKeyPath(t *testing.T) {
	t.Setenv("HOME", "/home/tester")

	tests := []struct{ key, want string }{
		{"id_work", "/home/tester/.ssh/id_work"},
		{"keys/id_work", "/home/tester/.ssh/keys/id_work"},
		{"./keys/id_work", "/home/tester/.ssh/keys/id_work"},
		{"~/.ssh/id_other", "/home/tester/.ssh/id_other"},
		{filepath.Join("/abs/path", "key"), "/abs/path/key"},
	}
	for _, tt := range tests {
		if got := KeyPath(tt.key); got != tt.want {
			t.Errorf("KeyPath(%q) = %q, want %q", tt.key, got, tt.want)
		}
	}
}

func TestSSHCommand(t *testing.T) {
	t.Setenv("HOME", "/home/tester")
	got := SSHCommand("id_work")
	want := "ssh -i /home/tester/.ssh/id_work -o IdentitiesOnly=yes"
	if got != want {
		t.Fatalf("SSHCommand = %q, want %q", got, want)
	}
}

func TestShellQuote(t *testing.T) {
	tests := []struct{ in, want string }{
		{"/home/u/.ssh/id_work", "/home/u/.ssh/id_work"},
		{"/home/my user/id_work", "'/home/my user/id_work'"},
		{"/home/it's/key", `'/home/it'\''s/key'`},
	}
	for _, tt := range tests {
		if got := shellQuote(tt.in); got != tt.want {
			t.Errorf("shellQuote(%q) = %q, want %q", tt.in, got, tt.want)
		}
	}
}

func TestApplyInitializesRepo(t *testing.T) {
	dir := t.TempDir()
	res, err := Apply(dir, Identity{Name: "Test User", Email: "test@example.com", Key: "id_test"})
	if err != nil {
		t.Fatalf("Apply: %v", err)
	}
	if !res.Initialized {
		t.Error("expected repository to be initialized")
	}
	if filepath.Base(res.Repo) != filepath.Base(dir) {
		t.Errorf("Repo = %q, want directory %q", res.Repo, dir)
	}

	for key, want := range map[string]string{
		"user.name":       "Test User",
		"user.email":      "test@example.com",
		"core.sshCommand": res.SSHCommand,
	} {
		got, err := git(dir, "config", "--local", "--get", key)
		if err != nil {
			t.Fatalf("reading %s: %v", key, err)
		}
		if got != want+"\n" {
			t.Errorf("%s = %q, want %q", key, got, want)
		}
	}
}
