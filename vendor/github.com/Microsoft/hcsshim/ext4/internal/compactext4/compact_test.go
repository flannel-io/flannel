package compactext4

import (
	"bytes"
	"encoding/binary"
	"fmt"
	"io"
	"os"
	"strings"
	"testing"
	"time"

	"github.com/Microsoft/hcsshim/ext4/internal/format"
)

type testFile struct {
	Path        string
	File        *File
	Data        []byte
	DataSize    int64
	Link        string
	ExpectError bool
}

var (
	data []byte
	name string
)

func init() {
	data = make([]byte, blockSize*2)
	for i := range data {
		data[i] = uint8(i)
	}

	nameb := make([]byte, 300)
	for i := range nameb {
		nameb[i] = byte('0' + i%10)
	}
	name = string(nameb)
}

type largeData struct {
	pos int64
}

func (d *largeData) Read(b []byte) (int, error) {
	p := d.pos
	var pb [8]byte
	for i := range b {
		binary.LittleEndian.PutUint64(pb[:], uint64(p+int64(i)))
		b[i] = pb[i%8]
	}
	p += int64(len(b))
	return len(b), nil
}

func (tf *testFile) Reader() io.Reader {
	if tf.DataSize != 0 {
		return io.LimitReader(&largeData{}, tf.DataSize)
	}
	return bytes.NewReader(tf.Data)
}

func createTestFile(t *testing.T, w *Writer, tf testFile) {
	var err error
	if tf.File != nil {
		tf.File.Size = int64(len(tf.Data))
		if tf.File.Size == 0 {
			tf.File.Size = tf.DataSize
		}
		err = w.Create(tf.Path, tf.File)
	} else {
		err = w.Link(tf.Link, tf.Path)
	}
	if tf.ExpectError && err == nil {
		t.Errorf("%s: expected error", tf.Path)
	} else if !tf.ExpectError && err != nil {
		t.Error(err)
	} else {
		_, err := io.Copy(w, tf.Reader())
		if err != nil {
			t.Error(err)
		}
	}
}

func expectedMode(f *File) uint16 {
	switch f.Mode & format.TypeMask {
	case 0:
		return f.Mode | S_IFREG
	case S_IFLNK:
		return f.Mode | 0777
	default:
		return f.Mode
	}
}

func expectedSize(f *File) int64 {
	switch f.Mode & format.TypeMask {
	case 0, S_IFREG:
		return f.Size
	case S_IFLNK:
		return int64(len(f.Linkname))
	default:
		return 0
	}
}

func xattrsEqual(x1, x2 map[string][]byte) bool {
	if len(x1) != len(x2) {
		return false
	}
	for name, value := range x1 {
		if !bytes.Equal(x2[name], value) {
			return false
		}
	}
	return true
}

func fileEqual(f1, f2 *File) bool {
	return f1.Linkname == f2.Linkname &&
		expectedSize(f1) == expectedSize(f2) &&
		expectedMode(f1) == expectedMode(f2) &&
		f1.Uid == f2.Uid &&
		f1.Gid == f2.Gid &&
		f1.Atime.Equal(f2.Atime) &&
		f1.Ctime.Equal(f2.Ctime) &&
		f1.Mtime.Equal(f2.Mtime) &&
		f1.Crtime.Equal(f2.Crtime) &&
		f1.Devmajor == f2.Devmajor &&
		f1.Devminor == f2.Devminor &&
		xattrsEqual(f1.Xattrs, f2.Xattrs)
}

func runTestsOnFiles(t *testing.T, testFiles []testFile, opts ...Option) {
	image := "testfs.img"
	imagef, err := os.Create(image)
	if err != nil {
		t.Fatal(err)
	}
	defer os.Remove(image)
	defer imagef.Close()

	w := NewWriter(imagef, opts...)
	for _, tf := range testFiles {
		createTestFile(t, w, tf)
		if !tf.ExpectError && tf.File != nil {
			f, err := w.Stat(tf.Path)
			if err != nil {
				if !strings.Contains(err.Error(), "cannot retrieve") {
					t.Error(err)
				}
			} else if !fileEqual(f, tf.File) {
				t.Errorf("%s: stat mismatch: %#v %#v", tf.Path, tf.File, f)
			}
		}
	}

	if t.Failed() {
		return
	}

	if err := w.Close(); err != nil {
		t.Fatal(err)
	}

	fsck(t, image)

	mountPath := "testmnt"

	if mountImage(t, image, mountPath) {
		defer unmountImage(t, mountPath)
		validated := make(map[string]*testFile)
		for i := range testFiles {
			tf := testFiles[len(testFiles)-i-1]
			if validated[tf.Link] != nil {
				// The link target was subsequently replaced. Find the
				// earlier instance.
				for j := range testFiles[:len(testFiles)-i-1] {
					otf := testFiles[j]
					if otf.Path == tf.Link && !otf.ExpectError {
						tf = otf
						break
					}
				}
			}
			if !tf.ExpectError && validated[tf.Path] == nil {
				verifyTestFile(t, mountPath, tf)
				validated[tf.Path] = &tf
			}
		}
	}
}

func TestBasic(t *testing.T) {
	now := time.Now()
	testFiles := []testFile{
		{Path: "empty", File: &File{Mode: 0644}},
		{Path: "small", File: &File{Mode: 0644}, Data: data[:40]},
		{Path: "time", File: &File{Atime: now, Ctime: now.Add(time.Second), Mtime: now.Add(time.Hour)}},
		{Path: "block_1", File: &File{Mode: 0644}, Data: data[:blockSize]},
		{Path: "block_2", File: &File{Mode: 0644}, Data: data[:blockSize*2]},
		{Path: "symlink", File: &File{Linkname: "block_1", Mode: format.S_IFLNK}},
		{Path: "symlink_59", File: &File{Linkname: name[:59], Mode: format.S_IFLNK}},
		{Path: "symlink_60", File: &File{Linkname: name[:60], Mode: format.S_IFLNK}},
		{Path: "symlink_120", File: &File{Linkname: name[:120], Mode: format.S_IFLNK}},
		{Path: "symlink_300", File: &File{Linkname: name[:300], Mode: format.S_IFLNK}},
		{Path: "dir", File: &File{Mode: format.S_IFDIR | 0755}},
		{Path: "dir/fifo", File: &File{Mode: format.S_IFIFO}},
		{Path: "dir/sock", File: &File{Mode: format.S_IFSOCK}},
		{Path: "dir/blk", File: &File{Mode: format.S_IFBLK, Devmajor: 0x5678, Devminor: 0x1234}},
		{Path: "dir/chr", File: &File{Mode: format.S_IFCHR, Devmajor: 0x5678, Devminor: 0x1234}},
		{Path: "dir/hard_link", Link: "small"},
	}

	runTestsOnFiles(t, testFiles)
}

func TestLargeDirectory(t *testing.T) {
	testFiles := []testFile{
		{Path: "bigdir", File: &File{Mode: format.S_IFDIR | 0755}},
	}
	for i := 0; i < 50000; i++ {
		testFiles = append(testFiles, testFile{
			Path: fmt.Sprintf("bigdir/%d", i), File: &File{Mode: 0644},
		})
	}

	runTestsOnFiles(t, testFiles)
}

func TestInlineData(t *testing.T) {
	testFiles := []testFile{
		{Path: "inline_30", File: &File{Mode: 0644}, Data: data[:30]},
		{Path: "inline_60", File: &File{Mode: 0644}, Data: data[:60]},
		{Path: "inline_120", File: &File{Mode: 0644}, Data: data[:120]},
		{Path: "inline_full", File: &File{Mode: 0644}, Data: data[:inlineDataSize]},
		{Path: "block_min", File: &File{Mode: 0644}, Data: data[:inlineDataSize+1]},
	}

	runTestsOnFiles(t, testFiles, InlineData)
}

func TestXattrs(t *testing.T) {
	testFiles := []testFile{
		{Path: "withsmallxattrs",
			File: &File{
				Mode: format.S_IFREG | 0644,
				Xattrs: map[string][]byte{
					"user.foo": []byte("test"),
					"user.bar": []byte("test2"),
				},
			},
		},
		{Path: "withlargexattrs",
			File: &File{
				Mode: format.S_IFREG | 0644,
				Xattrs: map[string][]byte{
					"user.foo": data[:100],
					"user.bar": data[:50],
				},
			},
		},
	}
	runTestsOnFiles(t, testFiles)
}

func TestReplace(t *testing.T) {
	testFiles := []testFile{
		{Path: "lost+found", ExpectError: true, File: &File{}}, // can't change type
		{Path: "lost+found", File: &File{Mode: format.S_IFDIR | 0777}},

		{Path: "dir", File: &File{Mode: format.S_IFDIR | 0777}},
		{Path: "dir/file", File: &File{}},
		{Path: "dir", File: &File{Mode: format.S_IFDIR | 0700}},

		{Path: "file", File: &File{}},
		{Path: "file", File: &File{Mode: 0600}},
		{Path: "file2", File: &File{}},
		{Path: "link", Link: "file2"},
		{Path: "file2", File: &File{Mode: 0600}},

		{Path: "nolinks", File: &File{}},
		{Path: "nolinks", ExpectError: true, Link: "file"}, // would orphan nolinks

		{Path: "onelink", File: &File{}},
		{Path: "onelink2", Link: "onelink"},
		{Path: "onelink", Link: "file"},

		{Path: "", ExpectError: true, File: &File{}},
		{Path: "", ExpectError: true, Link: "file"},
		{Path: "", File: &File{Mode: format.S_IFDIR | 0777}},

		{Path: "smallxattr", File: &File{Xattrs: map[string][]byte{"user.foo": data[:4]}}},
		{Path: "smallxattr", File: &File{Xattrs: map[string][]byte{"user.foo": data[:8]}}},

		{Path: "smallxattr_delete", File: &File{Xattrs: map[string][]byte{"user.foo": data[:4]}}},
		{Path: "smallxattr_delete", File: &File{}},

		{Path: "largexattr", File: &File{Xattrs: map[string][]byte{"user.small": data[:8], "user.foo": data[:200]}}},
		{Path: "largexattr", File: &File{Xattrs: map[string][]byte{"user.small": data[:12], "user.foo": data[:400]}}},

		{Path: "largexattr", File: &File{Xattrs: map[string][]byte{"user.foo": data[:200]}}},
		{Path: "largexattr_delete", File: &File{}},
	}
	runTestsOnFiles(t, testFiles)
}

func TestTime(t *testing.T) {
	now := time.Now()
	now2 := fsTimeToTime(timeToFsTime(now))
	if now.UnixNano() != now2.UnixNano() {
		t.Fatalf("%s != %s", now, now2)
	}
}

func TestLargeFile(t *testing.T) {
	testFiles := []testFile{
		{Path: "small", File: &File{}, DataSize: 1024 * 1024},        // can't change type
		{Path: "medium", File: &File{}, DataSize: 200 * 1024 * 1024}, // can't change type
		{Path: "large", File: &File{}, DataSize: 600 * 1024 * 1024},  // can't change type
	}
	runTestsOnFiles(t, testFiles)
}

func TestFileLinkLimit(t *testing.T) {
	testFiles := []testFile{
		{Path: "file", File: &File{}},
	}
	for i := 0; i < format.MaxLinks; i++ {
		testFiles = append(testFiles, testFile{Path: fmt.Sprintf("link%d", i), Link: "file"})
	}
	testFiles[len(testFiles)-1].ExpectError = true
	runTestsOnFiles(t, testFiles)
}

func TestDirLinkLimit(t *testing.T) {
	testFiles := []testFile{
		{Path: "dir", File: &File{Mode: S_IFDIR}},
	}
	for i := 0; i < format.MaxLinks-1; i++ {
		testFiles = append(testFiles, testFile{Path: fmt.Sprintf("dir/%d", i), File: &File{Mode: S_IFDIR}})
	}
	testFiles[len(testFiles)-1].ExpectError = true
	runTestsOnFiles(t, testFiles)
}

func TestLargeDisk(t *testing.T) {
	testFiles := []testFile{
		{Path: "file", File: &File{}},
	}
	runTestsOnFiles(t, testFiles, MaximumDiskSize(maxMaxDiskSize))
}
