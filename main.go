// Command clitor applies a git identity (user.name, user.email and a per-key
// core.sshCommand) to the current repository.
package main

import (
	"errors"
	"flag"
	"fmt"
	iofs "io/fs"
	"os"
	"strings"

	"clitor/internal/config"
	"clitor/internal/gitcfg"
	"clitor/internal/picker"
)

// errConfigCreated reports that the default config was just generated; the
// message has already been printed to the user.
var errConfigCreated = errors.New("конфиг создан")

func main() {
	if err := run(os.Args[1:]); err != nil {
		if errors.Is(err, flag.ErrHelp) {
			return
		}
		if errors.Is(err, errConfigCreated) {
			os.Exit(1)
		}
		if errors.Is(err, picker.ErrCanceled) {
			fmt.Fprintln(os.Stderr, "clitor: выбор отменён")
			os.Exit(130)
		}
		fmt.Fprintln(os.Stderr, "clitor:", err)
		os.Exit(1)
	}
}

func run(args []string) error {
	fs := flag.NewFlagSet("clitor", flag.ContinueOnError)
	fs.SetOutput(os.Stderr)
	configPath := fs.String("config", "", "путь к файлу конфигурации")
	list := fs.Bool("list", false, "показать доступные профили и выйти")
	fs.Usage = func() {
		out := fs.Output()
		fmt.Fprint(out, `clitor — настраивает git-конфиг в текущем репозитории

Использование:
  clitor                выбрать профиль в интерактивном меню
  clitor <name>         применить профиль по имени
  clitor -list          показать доступные профили

Флаги:
  -config PATH          путь к конфигу (по умолчанию
                        $XDG_CONFIG_HOME/clitor/config.yaml)
  -list                 показать профили и выйти

Пример:
  clitor bill

`)
	}
	if err := fs.Parse(args); err != nil {
		return err
	}

	if *configPath == "" {
		path, created, err := config.EnsureDefault()
		if err != nil {
			return err
		}
		if created {
			fmt.Fprintf(os.Stderr, "Создан конфиг: %s\nОтредактируйте его (раздел users) и запустите clitor снова.\n", path)
			return errConfigCreated
		}
	}

	cfg, err := config.Load(*configPath)
	if err != nil {
		return err
	}
	names := cfg.Names()

	if *list {
		for _, name := range names {
			fmt.Fprintln(os.Stdout, profileLabel(name, cfg.Profiles[name]))
		}
		return nil
	}

	rest := fs.Args()
	if len(rest) > 1 {
		return fmt.Errorf("ожидается не больше одного имени профиля (см. clitor -list)")
	}

	var chosen string
	if len(rest) == 1 {
		chosen = rest[0]
	} else {
		labels := make([]string, len(names))
		for i, name := range names {
			labels[i] = profileLabel(name, cfg.Profiles[name])
		}
		idx, err := picker.Pick("Выберите профиль ("+cfg.Path+"):", labels)
		if err != nil {
			return err
		}
		chosen = names[idx]
	}

	profile, ok := cfg.Profiles[chosen]
	if !ok {
		return fmt.Errorf("профиль %q не найден; доступны: %s", chosen, strings.Join(names, ", "))
	}

	dir, err := os.Getwd()
	if err != nil {
		return err
	}

	keyPath := gitcfg.KeyPath(profile.Key)
	info, err := os.Stat(keyPath)
	switch {
	case errors.Is(err, iofs.ErrNotExist):
		return fmt.Errorf("ssh-ключ не найден: %s\nпрофиль %q не применён", keyPath, chosen)
	case err != nil:
		return fmt.Errorf("не удалось проверить ssh-ключ %s: %w", keyPath, err)
	case info.IsDir():
		return fmt.Errorf("путь к ssh-ключу указывает на каталог: %s\nпрофиль %q не применён", keyPath, chosen)
	}

	res, err := gitcfg.Apply(dir, gitcfg.Identity{
		Name:  profile.Name,
		Email: profile.Email,
		Key:   profile.Key,
	})
	if err != nil {
		return err
	}

	fmt.Fprintf(os.Stdout, "Репозиторий: %s\n", res.Repo)
	if res.Initialized {
		fmt.Fprintln(os.Stdout, "git init: репозиторий создан")
	}
	fmt.Fprintf(os.Stdout, "  user.name       = %s\n", profile.Name)
	fmt.Fprintf(os.Stdout, "  user.email      = %s\n", profile.Email)
	fmt.Fprintf(os.Stdout, "  core.sshCommand = %s\n", res.SSHCommand)
	fmt.Fprintf(os.Stdout, "\nПрименён профиль %q.\n", chosen)
	return nil
}

// profileLabel renders a profile for -list and the selector. A missing ssh
// key is highlighted in red when output goes to a terminal.
func profileLabel(name string, p config.Profile) string {
	key := p.Key
	if keyMissing(p) {
		key = paintRed(key)
	}
	return fmt.Sprintf("%-12s %s <%s>  %s", name, p.Name, p.Email, key)
}

func keyMissing(p config.Profile) bool {
	info, err := os.Stat(gitcfg.KeyPath(p.Key))
	return err != nil || info.IsDir()
}

func paintRed(s string) string {
	if !colorEnabled() {
		return s
	}
	return "\x1b[31m" + s + "\x1b[0m"
}

func colorEnabled() bool {
	if _, ok := os.LookupEnv("NO_COLOR"); ok {
		return false
	}
	if os.Getenv("TERM") == "dumb" {
		return false
	}
	return picker.IsTerminal(int(os.Stdout.Fd()))
}
