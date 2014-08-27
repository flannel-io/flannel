package ip

import (
	"bytes"
	"fmt"
	"os"
	"syscall"
	"unsafe"

	"github.com/coreos/rudder/Godeps/_workspace/src/github.com/docker/libcontainer/netlink"
)

const (
	tunDevice = "/dev/net/tun"
)

type ifreqFlags struct {
	IfrnName  [netlink.IFNAMSIZ]byte
	IfruFlags uint16
}

func ioctl(fd int, request, argp uintptr) error {
	_, _, errno := syscall.Syscall(syscall.SYS_IOCTL, uintptr(fd), request, argp)
	if errno != 0 {
		return fmt.Errorf("ioctl failed with '%s'", errno)
	}
	return nil
}

func fromZeroTerm(s []byte) string {
	return string(bytes.TrimRight(s, "\000"))
}

func OpenTun(name string) (*os.File, string, error) {
	tun, err := os.OpenFile(tunDevice, os.O_RDWR, 0)
	if err != nil {
		return nil, "", err
	}

	var ifr ifreqFlags
	copy(ifr.IfrnName[:len(ifr.IfrnName)-1], []byte(name+"\000"))
	ifr.IfruFlags = syscall.IFF_TUN | syscall.IFF_NO_PI

	err = ioctl(int(tun.Fd()), syscall.TUNSETIFF, uintptr(unsafe.Pointer(&ifr)))
	if err != nil {
		return nil, "", err
	}

	ifname := fromZeroTerm(ifr.IfrnName[:netlink.IFNAMSIZ])
	return tun, ifname, nil
}
