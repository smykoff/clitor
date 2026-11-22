// Package picker implements a small interactive list selector.
package picker

import (
	"bufio"
	"errors"
	"fmt"
	"io"
	"os"
	"strconv"
	"strings"
)

// ErrCanceled is returned when the user aborts the selection.
var ErrCanceled = errors.New("выбор отменён")

// IsTerminal reports whether fd is a terminal.
func IsTerminal(fd int) bool { return isTerminal(fd) }

// Pick shows items and returns the index of the selected one. It uses an
// arrow-key selector on a terminal and a numbered prompt otherwise.
func Pick(title string, items []string) (int, error) {
	if len(items) == 0 {
		return 0, errors.New("нет доступных вариантов")
	}
	if isTerminal(int(os.Stdin.Fd())) && isTerminal(int(os.Stdout.Fd())) {
		if state, err := makeRaw(int(os.Stdin.Fd())); err == nil {
			defer state.restore()
			return pickKeys(title, items)
		}
	}
	return pickNumbered(title, items)
}

func pickKeys(title string, items []string) (int, error) {
	out := os.Stdout
	in := bufio.NewReader(os.Stdin)
	cursor := 0
	drawn := false
	frameLines := len(items) + 3 // title, blank, items, hint

	fmt.Fprint(out, "\x1b[?25l") // hide cursor
	defer func() {
		if drawn {
			fmt.Fprintf(out, "\x1b[%dA", frameLines)
			fmt.Fprint(out, "\x1b[J")
		}
		fmt.Fprint(out, "\x1b[?25h") // show cursor
	}()

	draw := func() {
		if drawn {
			fmt.Fprintf(out, "\x1b[%dA", frameLines)
		}
		fmt.Fprintf(out, "\x1b[2K%s\r\n\r\n", title)
		for i, item := range items {
			if i == cursor {
				fmt.Fprintf(out, "\x1b[2K\x1b[1m> %s\x1b[0m\r\n", item)
			} else {
				fmt.Fprintf(out, "\x1b[2K  %s\r\n", item)
			}
		}
		fmt.Fprint(out, "\x1b[2K  \x1b[2m↑/↓ или j/k — выбор, Enter — применить, q — отмена\x1b[0m\r\n")
		drawn = true
	}

	move := func(delta int) {
		next := cursor + delta
		if next < 0 {
			next = 0
		}
		if next > len(items)-1 {
			next = len(items) - 1
		}
		cursor = next
	}

	draw()
	for {
		b, err := in.ReadByte()
		if err != nil {
			if errors.Is(err, io.EOF) {
				return 0, ErrCanceled
			}
			return 0, err
		}
		switch b {
		case '\r', '\n':
			return cursor, nil
		case 3, 'q', 'Q': // Ctrl+C, q
			return 0, ErrCanceled
		case 27: // escape sequence, e.g. arrows
			b2, err := in.ReadByte()
			if err != nil || b2 != '[' {
				return 0, ErrCanceled // plain Esc cancels
			}
			b3, err := in.ReadByte()
			if err != nil {
				return 0, ErrCanceled
			}
			switch b3 {
			case 'A':
				move(-1)
			case 'B':
				move(1)
			case 'H':
				cursor = 0
			case 'F':
				cursor = len(items) - 1
			}
		case 'k':
			move(-1)
		case 'j':
			move(1)
		}
		draw()
	}
}

func pickNumbered(title string, items []string) (int, error) {
	fmt.Fprintln(os.Stdout, title)
	for i, item := range items {
		fmt.Fprintf(os.Stdout, "  %d) %s\n", i+1, item)
	}
	fmt.Fprint(os.Stdout, "Номер (Enter — отмена): ")

	line, err := bufio.NewReader(os.Stdin).ReadString('\n')
	if err != nil && !errors.Is(err, io.EOF) {
		return 0, err
	}
	line = strings.TrimSpace(line)
	if line == "" {
		return 0, ErrCanceled
	}
	n, err := strconv.Atoi(line)
	if err != nil || n < 1 || n > len(items) {
		return 0, fmt.Errorf("неверный выбор: %q", line)
	}
	return n - 1, nil
}
