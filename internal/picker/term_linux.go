//go:build linux

package picker

import (
	"syscall"
	"unsafe"
)

type terminalState struct {
	fd  int
	old syscall.Termios
}

func ioctl(fd int, req uintptr, arg unsafe.Pointer) error {
	_, _, errno := syscall.Syscall6(syscall.SYS_IOCTL, uintptr(fd), req, uintptr(arg), 0, 0, 0)
	if errno != 0 {
		return errno
	}
	return nil
}

func isTerminal(fd int) bool {
	var t syscall.Termios
	return ioctl(fd, syscall.TCGETS, unsafe.Pointer(&t)) == nil
}

// makeRaw switches the terminal to non-canonical, no-echo mode so that single
// key presses (including arrow keys) can be read without waiting for Enter.
func makeRaw(fd int) (*terminalState, error) {
	var old syscall.Termios
	if err := ioctl(fd, syscall.TCGETS, unsafe.Pointer(&old)); err != nil {
		return nil, err
	}

	raw := old
	raw.Iflag &^= syscall.IGNBRK | syscall.BRKINT | syscall.PARMRK | syscall.ISTRIP |
		syscall.INLCR | syscall.IGNCR | syscall.ICRNL | syscall.IXON
	raw.Oflag &^= syscall.OPOST
	raw.Lflag &^= syscall.ECHO | syscall.ECHONL | syscall.ICANON | syscall.ISIG | syscall.IEXTEN
	raw.Cflag &^= syscall.CSIZE | syscall.PARENB
	raw.Cflag |= syscall.CS8
	raw.Cc[syscall.VMIN] = 1
	raw.Cc[syscall.VTIME] = 0

	if err := ioctl(fd, syscall.TCSETS, unsafe.Pointer(&raw)); err != nil {
		return nil, err
	}
	return &terminalState{fd: fd, old: old}, nil
}

func (s *terminalState) restore() {
	_ = ioctl(s.fd, syscall.TCSETS, unsafe.Pointer(&s.old))
}
