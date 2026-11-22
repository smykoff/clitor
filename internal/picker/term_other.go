//go:build !linux

package picker

import "errors"

type terminalState struct{}

func isTerminal(fd int) bool { return false }

func makeRaw(fd int) (*terminalState, error) {
	return nil, errors.New("raw mode недоступен на этой платформе")
}

func (s *terminalState) restore() {}
