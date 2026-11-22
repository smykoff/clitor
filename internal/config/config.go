// Package config loads clitor user profiles from a small YAML file.
package config

import (
	"errors"
	"fmt"
	"io/fs"
	"os"
	"path/filepath"
	"sort"
	"strings"
)

// Filesystem seams, overridden in tests so they never touch the real disk.
var (
	readFile      = os.ReadFile
	writeFile     = os.WriteFile
	statPath      = os.Stat
	mkdirAll      = os.MkdirAll
	userConfigDir = os.UserConfigDir
)

// Profile is a single git identity that can be applied to a repository.
type Profile struct {
	Name  string
	Email string
	Key   string
}

// Config is a parsed configuration file.
type Config struct {
	// Path is the file the configuration was read from.
	Path string
	// Profiles maps a profile name (as used by `clitor <name>`) to its identity.
	Profiles map[string]Profile
}

// Load reads the configuration from explicitPath, or from the default XDG
// location when explicitPath is empty. See Resolve.
func Load(explicitPath string) (*Config, error) {
	path, err := Resolve(explicitPath)
	if err != nil {
		return nil, err
	}
	data, err := readFile(path)
	if err != nil {
		return nil, fmt.Errorf("не удалось прочитать конфиг: %w", err)
	}
	profiles, err := Parse(data)
	if err != nil {
		return nil, fmt.Errorf("%s: %w", path, err)
	}
	for name, p := range profiles {
		switch {
		case p.Name == "":
			return nil, fmt.Errorf("%s: у профиля %q не задано поле name", path, name)
		case p.Email == "":
			return nil, fmt.Errorf("%s: у профиля %q не задано поле email", path, name)
		case p.Key == "":
			return nil, fmt.Errorf("%s: у профиля %q не задано поле key", path, name)
		}
	}
	return &Config{Path: path, Profiles: profiles}, nil
}

// DefaultPath returns the default configuration path:
// $XDG_CONFIG_HOME/clitor/config.yaml (usually ~/.config/clitor/config.yaml).
func DefaultPath() (string, error) {
	dir, err := userConfigDir()
	if err != nil {
		return "", fmt.Errorf("не удалось определить каталог конфигурации (XDG_CONFIG_HOME или HOME): %w", err)
	}
	return filepath.Join(dir, "clitor", "config.yaml"), nil
}

// Resolve returns the configuration path. An explicit path wins; otherwise the
// default XDG path is used and its absence is an error.
func Resolve(explicitPath string) (string, error) {
	if explicitPath != "" {
		abs, err := filepath.Abs(explicitPath)
		if err != nil {
			return "", err
		}
		if _, err := statPath(abs); err != nil {
			return "", fmt.Errorf("конфиг не найден: %s", explicitPath)
		}
		return abs, nil
	}

	path, err := DefaultPath()
	if err != nil {
		return "", err
	}
	info, err := statPath(path)
	if err != nil || info.IsDir() {
		return "", fmt.Errorf("конфиг не найден: %s\nсоздайте его или укажите путь через -config", path)
	}
	return path, nil
}

// EnsureDefault makes sure the default configuration file exists, creating it
// with a commented-out template when it is missing. It returns the path and
// whether the file was created.
func EnsureDefault() (path string, created bool, err error) {
	path, err = DefaultPath()
	if err != nil {
		return "", false, err
	}

	if _, statErr := statPath(path); statErr == nil {
		return path, false, nil
	} else if !errors.Is(statErr, fs.ErrNotExist) {
		return "", false, fmt.Errorf("не удалось проверить конфиг %s: %w", path, statErr)
	}

	if err := mkdirAll(filepath.Dir(path), 0o700); err != nil {
		return "", false, fmt.Errorf("не удалось создать каталог конфига: %w", err)
	}
	if err := writeFile(path, []byte(defaultTemplate), 0o600); err != nil {
		return "", false, fmt.Errorf("не удалось создать конфиг %s: %w", path, err)
	}
	return path, true, nil
}

// defaultTemplate is written on the first run. Everything is commented out so
// that clitor never applies a placeholder identity by accident.
const defaultTemplate = `# clitor — профили git-идентичностей.
#
# Заполните свои данные и уберите комментарии:
#
# users:
#   work:
#     name: Иван Петров
#     email: ivan@company.com
#     key: id_work
#
# Поле key — имя файла в ~/.ssh, абсолютный путь или путь вида ~/.ssh/key.
`

// Names returns the profile names in a stable (alphabetical) order.
func (c *Config) Names() []string {
	out := make([]string, 0, len(c.Profiles))
	for name := range c.Profiles {
		out = append(out, name)
	}
	sort.Strings(out)
	return out
}

// Parse decodes the subset of YAML clitor needs:
//
//	users:
//	  profile-name:
//	    name: ...
//	    email: ...
//	    key: ...
func Parse(data []byte) (map[string]Profile, error) {
	profiles := make(map[string]Profile)
	seenFields := make(map[string]map[string]bool)

	const (
		topLevel = 0
		profile  = 2
		field    = 4
	)

	inUsers := false
	current := ""

	for i, raw := range strings.Split(string(data), "\n") {
		lineNo := i + 1
		if i == 0 {
			raw = strings.TrimPrefix(raw, "\ufeff")
		}
		line := stripComment(raw)
		if strings.TrimSpace(line) == "" {
			continue
		}
		if strings.ContainsRune(line, '\t') {
			return nil, fmt.Errorf("строка %d: табуляция не поддерживается, используйте пробелы", lineNo)
		}

		indent := len(line) - len(strings.TrimLeft(line, " "))
		key, value, ok := splitKeyValue(strings.TrimSpace(line))
		if !ok {
			return nil, fmt.Errorf("строка %d: ожидалось «ключ: значение»", lineNo)
		}
		key = unquote(key)

		switch {
		case indent == topLevel:
			if key != "users" {
				return nil, fmt.Errorf("строка %d: неизвестный раздел %q (ожидался users)", lineNo, key)
			}
			if value != "" {
				return nil, fmt.Errorf("строка %d: users должен быть блоком", lineNo)
			}
			inUsers = true

		case inUsers && indent == profile:
			if value != "" {
				return nil, fmt.Errorf("строка %d: у профиля %q не может быть значения в строке", lineNo, key)
			}
			if _, exists := profiles[key]; exists {
				return nil, fmt.Errorf("строка %d: профиль %q объявлен дважды", lineNo, key)
			}
			profiles[key] = Profile{}
			seenFields[key] = make(map[string]bool)
			current = key

		case inUsers && indent == field:
			if current == "" {
				return nil, fmt.Errorf("строка %d: поле %q вне профиля", lineNo, key)
			}
			if seenFields[current][key] {
				return nil, fmt.Errorf("строка %d: поле %q профиля %q указано дважды", lineNo, key, current)
			}
			seenFields[current][key] = true

			p := profiles[current]
			switch key {
			case "name":
				p.Name = unquote(value)
			case "email":
				p.Email = unquote(value)
			case "key":
				p.Key = unquote(value)
			default:
				return nil, fmt.Errorf("строка %d: неизвестное поле %q (допустимы name, email, key)", lineNo, key)
			}
			profiles[current] = p

		default:
			return nil, fmt.Errorf("строка %d: неожиданный отступ (%d пробелов)", lineNo, indent)
		}
	}

	if len(profiles) == 0 {
		return nil, fmt.Errorf("в конфиге нет ни одного профиля в разделе users")
	}
	return profiles, nil
}

// stripComment removes a trailing # comment that is outside quotes.
func stripComment(line string) string {
	var quote byte
	for i := 0; i < len(line); i++ {
		c := line[i]
		switch {
		case quote != 0:
			if c == quote {
				quote = 0
			}
		case c == '\'' || c == '"':
			quote = c
		case c == '#' && (i == 0 || line[i-1] == ' ' || line[i-1] == '\t'):
			return line[:i]
		}
	}
	return line
}

// splitKeyValue splits "key: value" at the first colon outside quotes.
func splitKeyValue(s string) (key, value string, ok bool) {
	var quote byte
	for i := 0; i < len(s); i++ {
		c := s[i]
		switch {
		case quote != 0:
			if c == quote {
				quote = 0
			}
		case c == '\'' || c == '"':
			quote = c
		case c == ':':
			if i+1 == len(s) || s[i+1] == ' ' || s[i+1] == '\t' {
				return strings.TrimSpace(s[:i]), strings.TrimSpace(s[i+1:]), true
			}
		}
	}
	return "", "", false
}

func unquote(s string) string {
	if len(s) >= 2 && (s[0] == '\'' || s[0] == '"') && s[len(s)-1] == s[0] {
		return s[1 : len(s)-1]
	}
	return s
}
