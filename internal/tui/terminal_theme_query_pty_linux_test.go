package tui

import (
	"strconv"

	"golang.org/x/sys/unix"
)

// openPTY opens a pseudo-terminal pair: master and slave descriptors.
func openPTY() (master, slave int, err error) {
	master, err = unix.Open("/dev/ptmx", unix.O_RDWR|unix.O_NOCTTY, 0)
	if err != nil {
		return 0, 0, err
	}
	var n uint32
	if err = unix.IoctlSetPointerInt(master, unix.TIOCSPTLCK, 0); err == nil {
		n, err = unix.IoctlGetUint32(master, unix.TIOCGPTN)
	}
	if err == nil {
		slave, err = unix.Open("/dev/pts/"+strconv.Itoa(int(n)), unix.O_RDWR|unix.O_NOCTTY, 0)
	}
	if err != nil {
		unix.Close(master)
		return 0, 0, err
	}
	return master, slave, nil
}
