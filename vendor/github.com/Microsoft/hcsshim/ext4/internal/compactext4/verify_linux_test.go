package compactext4

import (
	"bytes"
	"fmt"
	"io"
	"os"
	"os/exec"
	"path"
	"syscall"
	"testing"
	"time"
	"unsafe"

	"github.com/Microsoft/hcsshim/ext4/internal/format"
)

func timeEqual(ts syscall.Timespec, t time.Time) bool {
	sec, nsec := t.Unix(), t.Nanosecond()
	if t.IsZero() {
		sec, nsec = 0, 0
	}
	return ts.Sec == sec && int(ts.Nsec) == nsec
}

func expectedDevice(f *File) uint64 {
	return uint64(f.Devminor&0xff | f.Devmajor<<8 | (f.Devminor&0xffffff00)<<12)
}

func llistxattr(path string, b []byte) (int, error) {
	pathp := syscall.StringBytePtr(path)
	var p unsafe.Pointer
	if len(b) > 0 {
		p = unsafe.Pointer(&b[0])
	}
	r, _, e := syscall.Syscall(syscall.SYS_LLISTXATTR, uintptr(unsafe.Pointer(pathp)), uintptr(p), uintptr(len(b)))
	if e != 0 {
		return 0, &os.PathError{Path: path, Op: "llistxattr", Err: syscall.Errno(e)}
	}
	return int(r), nil
}

func lgetxattr(path string, name string, b []byte) (int, error) {
	pathp := syscall.StringBytePtr(path)
	namep := syscall.StringBytePtr(name)
	var p unsafe.Pointer
	if len(b) > 0 {
		p = unsafe.Pointer(&b[0])
	}
	r, _, e := syscall.Syscall6(syscall.SYS_LGETXATTR, uintptr(unsafe.Pointer(pathp)), uintptr(unsafe.Pointer(namep)), uintptr(p), uintptr(len(b)), 0, 0)
	if e != 0 {
		return 0, &os.PathError{Path: path, Op: "lgetxattr", Err: syscall.Errno(e)}
	}
	return int(r), nil
}

func readXattrs(path string) (map[string][]byte, error) {
	xattrs := make(map[string][]byte)
	var buf [4096]byte
	var buf2 [4096]byte
	b := buf[:]
	n, err := llistxattr(path, b)
	if err != nil {
		return nil, err
	}
	b = b[:n]
	for len(b) != 0 {
		nn := bytes.IndexByte(b, 0)
		name := string(b[:nn])
		b = b[nn+1:]
		vn, err := lgetxattr(path, name, buf2[:])
		if err != nil {
			return nil, err
		}
		value := buf2[:vn]
		xattrs[name] = value
	}
	return xattrs, nil
}

func streamEqual(r1, r2 io.Reader) (bool, error) {
	var b [4096]byte
	var b2 [4096]byte
	for {
		n, err := r1.Read(b[:])
		if n == 0 {
			if err == io.EOF {
				break
			}
			if err == nil {
				continue
			}
			return false, err
		}
		_, err = io.ReadFull(r2, b2[:n])
		if err == io.EOF || err == io.ErrUnexpectedEOF {
			return false, nil
		}
		if err != nil {
			return false, err
		}
		if !bytes.Equal(b[n:], b2[n:]) {
			return false, nil
		}
	}
	// Check the tail of r2
	_, err := r2.Read(b[:1])
	if err == nil {
		return false, nil
	}
	if err != io.EOF {
		return false, err
	}
	return true, nil
}

func verifyTestFile(t *testing.T, mountPath string, tf testFile) {
	name := path.Join(mountPath, tf.Path)
	fi, err := os.Lstat(name)
	if err != nil {
		t.Error(err)
		return
	}
	st := fi.Sys().(*syscall.Stat_t)
	if tf.File != nil {
		if st.Mode != uint32(expectedMode(tf.File)) ||
			st.Uid != tf.File.Uid ||
			st.Gid != tf.File.Gid ||
			(!fi.IsDir() && st.Size != expectedSize(tf.File)) ||
			st.Rdev != expectedDevice(tf.File) ||
			!timeEqual(st.Atim, tf.File.Atime) ||
			!timeEqual(st.Mtim, tf.File.Mtime) ||
			!timeEqual(st.Ctim, tf.File.Ctime) {

			t.Errorf("%s: stat mismatch, expected: %#v got: %#v", tf.Path, tf.File, st)
		}

		xattrs, err := readXattrs(name)
		if err != nil {
			t.Error(err)
		} else if !xattrsEqual(xattrs, tf.File.Xattrs) {
			t.Errorf("%s: xattr mismatch, expected: %#v got: %#v", tf.Path, tf.File.Xattrs, xattrs)
		}

		switch tf.File.Mode & format.TypeMask {
		case S_IFREG:
			if f, err := os.Open(name); err != nil {
				t.Error(err)
			} else {
				same, err := streamEqual(f, tf.Reader())
				if err != nil {
					t.Error(err)
				} else if !same {
					t.Errorf("%s: data mismatch", tf.Path)
				}
				f.Close()
			}
		case S_IFLNK:
			if link, err := os.Readlink(name); err != nil {
				t.Error(err)
			} else if link != tf.File.Linkname {
				t.Errorf("%s: link mismatch, expected: %s got: %s", tf.Path, tf.File.Linkname, link)
			}
		}
	} else {
		lfi, err := os.Lstat(path.Join(mountPath, tf.Link))
		if err != nil {
			t.Error(err)
			return
		}

		lst := lfi.Sys().(*syscall.Stat_t)
		if lst.Ino != st.Ino {
			t.Errorf("%s: hard link mismatch with %s, expected inode: %d got inode: %d", tf.Path, tf.Link, lst.Ino, st.Ino)
		}
	}
}

type capHeader struct {
	version uint32
	pid     int
}

type capData struct {
	effective   uint32
	permitted   uint32
	inheritable uint32
}

const CAP_SYS_ADMIN = 21

type caps struct {
	hdr  capHeader
	data [2]capData
}

func getCaps() (caps, error) {
	var c caps

	// Get capability version
	if _, _, errno := syscall.Syscall(syscall.SYS_CAPGET, uintptr(unsafe.Pointer(&c.hdr)), uintptr(unsafe.Pointer(nil)), 0); errno != 0 {
		return c, fmt.Errorf("SYS_CAPGET: %v", errno)
	}

	// Get current capabilities
	if _, _, errno := syscall.Syscall(syscall.SYS_CAPGET, uintptr(unsafe.Pointer(&c.hdr)), uintptr(unsafe.Pointer(&c.data[0])), 0); errno != 0 {
		return c, fmt.Errorf("SYS_CAPGET: %v", errno)
	}

	return c, nil
}

func mountImage(t *testing.T, image string, mountPath string) bool {
	caps, err := getCaps()
	if err != nil || caps.data[0].effective&(1<<uint(CAP_SYS_ADMIN)) == 0 {
		t.Log("cannot mount to run verification tests without CAP_SYS_ADMIN")
		return false
	}

	err = os.MkdirAll(mountPath, 0777)
	if err != nil {
		t.Fatal(err)
	}

	out, err := exec.Command("mount", "-o", "loop,ro", "-t", "ext4", image, mountPath).CombinedOutput()
	t.Logf("%s", out)
	if err != nil {
		t.Fatal(err)
	}
	return true
}

func unmountImage(t *testing.T, mountPath string) {
	out, err := exec.Command("umount", mountPath).CombinedOutput()
	t.Logf("%s", out)
	if err != nil {
		t.Log(err)
	}
}

func fsck(t *testing.T, image string) {
	cmd := exec.Command("e2fsck", "-v", "-f", "-n", image)
	out, err := cmd.CombinedOutput()
	t.Logf("%s", out)
	if err != nil {
		t.Fatal(err)
	}
}
