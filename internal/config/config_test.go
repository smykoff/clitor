package config

import (
	"io/fs"
	"os"
	"path/filepath"
	"strings"
	"testing"
	"time"
)

const sample = `users:
  alice:
    name: Alice
    email: alice@example.com
    key: id_alice

  bob:
    name: Bob
    email: bob@example.com
    key: id_bob

  carol:
    name: Carol
    email: carol@example.com
    key: id_carol
`

func TestParse(t *testing.T) {
	profiles, err := Parse([]byte(sample))
	if err != nil {
		t.Fatalf("Parse: %v", err)
	}
	if len(profiles) != 3 {
		t.Fatalf("got %d profiles, want 3", len(profiles))
	}
	want := Profile{Name: "Alice", Email: "alice@example.com", Key: "id_alice"}
	if got := profiles["alice"]; got != want {
		t.Errorf("alice = %+v, want %+v", got, want)
	}
	if got := profiles["carol"]; got.Key != "id_carol" {
		t.Errorf("carol key = %q", got.Key)
	}
}

func TestParseQuotedValues(t *testing.T) {
	profiles, err := Parse([]byte(`users:
  "my profile":
    name: 'Ivan #1'
    email: "ivan@example.com"
    key: "id #2"
`))
	if err != nil {
		t.Fatalf("Parse: %v", err)
	}
	got := profiles["my profile"]
	if got.Name != "Ivan #1" || got.Key != "id #2" || got.Email != "ivan@example.com" {
		t.Fatalf("unexpected profile: %+v", got)
	}
}

func TestParseErrors(t *testing.T) {
	tests := []struct {
		name string
		in   string
		want string
	}{
		{"tab", "users:\n\talice:\n", "табуляция"},
		{"bad section", "people:\n  a:\n", "неизвестный раздел"},
		{"unknown field", "users:\n  a:\n    login: x\n", "неизвестное поле"},
		{"duplicate profile", "users:\n  a:\n    name: x\n  a:\n    name: y\n", "объявлен дважды"},
		{"duplicate field", "users:\n  a:\n    name: x\n    name: y\n", "указано дважды"},
		{"empty", "# nothing\n", "нет ни одного профиля"},
	}
	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			_, err := Parse([]byte(tt.in))
			if err == nil {
				t.Fatal("expected error")
			}
			if !strings.Contains(err.Error(), tt.want) {
				t.Fatalf("error %q does not contain %q", err, tt.want)
			}
		})
	}
}

func TestNamesSorted(t *testing.T) {
	profiles, err := Parse([]byte(sample))
	if err != nil {
		t.Fatal(err)
	}
	cfg := &Config{Profiles: profiles}
	got := strings.Join(cfg.Names(), ",")
	want := "alice,bob,carol"
	if got != want {
		t.Fatalf("Names = %q, want %q", got, want)
	}
}

// TestLoad covers reading and validating a config through a fake filesystem.
func TestLoad(t *testing.T) {
	t.Run("valid", func(t *testing.T) {
		fs := newFakeFS()
		fs.addFile("/cfg/config.yaml", sample)
		useFakeFS(t, fs, "/xdg")

		cfg, err := Load("/cfg/config.yaml")
		if err != nil {
			t.Fatalf("Load: %v", err)
		}
		if cfg.Path != "/cfg/config.yaml" {
			t.Errorf("Path = %q", cfg.Path)
		}
		if got := cfg.Profiles["alice"]; got.Email != "alice@example.com" {
			t.Errorf("unexpected profile: %+v", got)
		}
	})

	t.Run("missing key", func(t *testing.T) {
		fs := newFakeFS()
		fs.addFile("/cfg/config.yaml", "users:\n  a:\n    name: x\n    email: y\n")
		useFakeFS(t, fs, "/xdg")

		_, err := Load("/cfg/config.yaml")
		if err == nil || !strings.Contains(err.Error(), "key") {
			t.Fatalf("expected missing key error, got %v", err)
		}
	})

	t.Run("unreadable", func(t *testing.T) {
		fs := newFakeFS()
		fs.addUnreadable("/cfg/config.yaml")
		useFakeFS(t, fs, "/xdg")

		_, err := Load("/cfg/config.yaml")
		if err == nil || !strings.Contains(err.Error(), "не удалось прочитать конфиг") {
			t.Fatalf("expected read error, got %v", err)
		}
	})
}

func TestResolveExplicit(t *testing.T) {
	fs := newFakeFS()
	fs.addFile("/cfg/custom.yml", sample)
	useFakeFS(t, fs, "/xdg")

	got, err := Resolve("/cfg/custom.yml")
	if err != nil {
		t.Fatalf("Resolve: %v", err)
	}
	if got != "/cfg/custom.yml" {
		t.Fatalf("Resolve = %q, want %q", got, "/cfg/custom.yml")
	}
	if _, err := Resolve("/cfg/missing.yml"); err == nil {
		t.Fatal("expected error for missing explicit config")
	}
}

func TestResolveDefault(t *testing.T) {
	fs := newFakeFS()
	useFakeFS(t, fs, "/xdg")

	if _, err := Resolve(""); err == nil {
		t.Fatal("expected error when default config is missing")
	}

	want := filepath.Join("/xdg", "clitor", "config.yaml")
	fs.addFile(want, sample)

	got, err := Resolve("")
	if err != nil {
		t.Fatalf("Resolve: %v", err)
	}
	if got != want {
		t.Fatalf("Resolve = %q, want %q", got, want)
	}
}

// TestResolveDefaultIgnoresWorkingDirectory checks that the default lookup only
// ever probes $XDG_CONFIG_HOME/clitor/config.yaml, never a config file in the
// current directory.
func TestResolveDefaultIgnoresWorkingDirectory(t *testing.T) {
	fs := newFakeFS()
	fs.addFile("config.yml", sample)
	fs.addFile("config.yaml", sample)
	useFakeFS(t, fs, "/xdg")

	if _, err := Resolve(""); err == nil {
		t.Fatal("expected error when default config is missing")
	}
	for _, name := range fs.statCalls {
		if name == "config.yml" || name == "config.yaml" {
			t.Fatalf("Resolve probed working-directory file %q", name)
		}
	}
}

func TestDefaultPathError(t *testing.T) {
	useFakeFS(t, newFakeFS(), "/xdg")
	userConfigDir = func() (string, error) { return "", os.ErrNotExist }

	if _, err := DefaultPath(); err == nil {
		t.Fatal("expected error when config dir cannot be determined")
	}
}

func TestEnsureDefaultCreates(t *testing.T) {
	fs := newFakeFS()
	useFakeFS(t, fs, "/xdg")

	want := filepath.Join("/xdg", "clitor", "config.yaml")
	path, created, err := EnsureDefault()
	if err != nil {
		t.Fatalf("EnsureDefault: %v", err)
	}
	if !created {
		t.Fatal("expected created=true")
	}
	if path != want {
		t.Fatalf("path = %q, want %q", path, want)
	}
	content, ok := fs.writes[want]
	if !ok {
		t.Fatalf("config was not written to %q", want)
	}
	if string(content) != defaultTemplate {
		t.Errorf("unexpected template content:\n%s", content)
	}
	if fs.modes[want] != 0o600 {
		t.Errorf("mode = %o, want 600", fs.modes[want])
	}
	if !fs.dirs[filepath.Dir(want)] {
		t.Errorf("expected directory %q to be created", filepath.Dir(want))
	}

	_, created, err = EnsureDefault()
	if err != nil {
		t.Fatalf("second EnsureDefault: %v", err)
	}
	if created {
		t.Fatal("second call must not recreate the config")
	}
}

func TestEnsureDefaultKeepsExisting(t *testing.T) {
	fs := newFakeFS()
	path := filepath.Join("/xdg", "clitor", "config.yaml")
	fs.addFile(path, sample)
	useFakeFS(t, fs, "/xdg")

	got, created, err := EnsureDefault()
	if err != nil {
		t.Fatalf("EnsureDefault: %v", err)
	}
	if created || got != path {
		t.Fatalf("got (%q, %v), want (%q, false)", got, created, path)
	}
	if len(fs.writes) != 0 {
		t.Errorf("existing config must not be overwritten: %d writes", len(fs.writes))
	}
}

func TestEnsureDefaultErrors(t *testing.T) {
	t.Run("mkdir", func(t *testing.T) {
		fs := newFakeFS()
		fs.mkdirErr = os.ErrPermission
		useFakeFS(t, fs, "/xdg")

		if _, _, err := EnsureDefault(); err == nil {
			t.Fatal("expected mkdir error")
		}
	})

	t.Run("write", func(t *testing.T) {
		fs := newFakeFS()
		fs.writeErr = os.ErrPermission
		useFakeFS(t, fs, "/xdg")

		if _, _, err := EnsureDefault(); err == nil {
			t.Fatal("expected write error")
		}
	})
}

// fakeFS is an in-memory filesystem used instead of touching the real disk.
type fakeFS struct {
	files      map[string][]byte
	dirs       map[string]bool
	unreadable map[string]bool
	writes     map[string][]byte
	modes      map[string]os.FileMode
	statCalls  []string
	mkdirErr   error
	writeErr   error
}

func newFakeFS() *fakeFS {
	return &fakeFS{
		files:      make(map[string][]byte),
		dirs:       make(map[string]bool),
		unreadable: make(map[string]bool),
		writes:     make(map[string][]byte),
		modes:      make(map[string]os.FileMode),
	}
}

func (f *fakeFS) addFile(name, content string) { f.files[name] = []byte(content) }

// addUnreadable registers a path that exists but cannot be read.
func (f *fakeFS) addUnreadable(name string) { f.unreadable[name] = true }

func (f *fakeFS) read(name string) ([]byte, error) {
	if f.unreadable[name] {
		return nil, &fs.PathError{Op: "open", Path: name, Err: fs.ErrPermission}
	}
	if data, ok := f.files[name]; ok {
		return data, nil
	}
	return nil, &fs.PathError{Op: "open", Path: name, Err: fs.ErrNotExist}
}

func (f *fakeFS) write(name string, data []byte, perm os.FileMode) error {
	if f.writeErr != nil {
		return f.writeErr
	}
	f.writes[name] = data
	f.files[name] = data
	f.modes[name] = perm
	return nil
}

func (f *fakeFS) mkdirAll(name string, _ os.FileMode) error {
	if f.mkdirErr != nil {
		return f.mkdirErr
	}
	f.dirs[name] = true
	return nil
}

func (f *fakeFS) stat(name string) (fs.FileInfo, error) {
	f.statCalls = append(f.statCalls, name)
	if f.dirs[name] {
		return fakeInfo{name: filepath.Base(name), dir: true}, nil
	}
	if _, ok := f.files[name]; ok {
		return fakeInfo{name: filepath.Base(name)}, nil
	}
	if f.unreadable[name] {
		return fakeInfo{name: filepath.Base(name)}, nil
	}
	return nil, &fs.PathError{Op: "stat", Path: name, Err: fs.ErrNotExist}
}

type fakeInfo struct {
	name string
	dir  bool
}

func (i fakeInfo) Name() string       { return i.name }
func (i fakeInfo) Size() int64        { return 0 }
func (i fakeInfo) ModTime() time.Time { return time.Time{} }
func (i fakeInfo) Sys() any           { return nil }
func (i fakeInfo) IsDir() bool        { return i.dir }
func (i fakeInfo) Mode() fs.FileMode {
	if i.dir {
		return fs.ModeDir | 0o755
	}
	return 0o600
}

// useFakeFS swaps the package filesystem seams for f and restores them on cleanup.
func useFakeFS(t *testing.T, f *fakeFS, cfgDir string) {
	t.Helper()
	readFile, writeFile, statPath, mkdirAll = f.read, f.write, f.stat, f.mkdirAll
	userConfigDir = func() (string, error) { return cfgDir, nil }
	t.Cleanup(func() {
		readFile, writeFile, statPath, mkdirAll = os.ReadFile, os.WriteFile, os.Stat, os.MkdirAll
		userConfigDir = os.UserConfigDir
	})
}
