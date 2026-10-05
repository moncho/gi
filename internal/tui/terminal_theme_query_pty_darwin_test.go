package tui

import (
	"bytes"
	"unsafe"

	"golang.org/x/sys/unix"
)

// openPTY opens a pseudo-terminal pair: master and slave descriptors.
func openPTY() (master, slave int, err error) {
	master, err = unix.Open("/dev/ptmx", unix.O_RDWR|unix.O_NOCTTY, 0)
	if err != nil {
		return 0, 0, err
	}
	name := make([]byte, 128)
	if err = unix.IoctlSetInt(master, unix.TIOCPTYGRANT, 0); err == nil {
		err = unix.IoctlSetInt(master, unix.TIOCPTYUNLK, 0)
	}
	if err == nil {
		_, _, errno := unix.Syscall(unix.SYS_IOCTL, uintptr(master), uintptr(unix.TIOCPTYGNAME), uintptr(unsafe.Pointer(&name[0])))
		if errno != 0 {
			err = errno
		}
	}
	if err == nil {
		if i := bytes.IndexByte(name, 0); i >= 0 {
			name = name[:i]
		}
		slave, err = unix.Open(string(name), unix.O_RDWR|unix.O_NOCTTY, 0)
	}
	if err != nil {
		unix.Close(master)
		return 0, 0, err
	}
	return master, slave, nil
}
