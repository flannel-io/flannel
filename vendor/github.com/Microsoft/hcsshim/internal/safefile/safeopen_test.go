package safefile

import (
	"io/ioutil"
	"os"
	"path/filepath"
	"syscall"
	"testing"

	winio "github.com/Microsoft/go-winio"
)

func tempRoot() (*os.File, error) {
	name, err := ioutil.TempDir("", "hcsshim-test")
	if err != nil {
		return nil, err
	}
	f, err := OpenRoot(name)
	if err != nil {
		os.Remove(name)
		return nil, err
	}
	return f, nil
}

func TestRemoveRelativeReadOnly(t *testing.T) {
	root, err := tempRoot()
	if err != nil {
		t.Fatal(err)
	}
	defer os.RemoveAll(root.Name())
	defer root.Close()

	p := filepath.Join(root.Name(), "foo")
	f, err := os.Create(p)
	if err != nil {
		t.Fatal(err)
	}
	defer f.Close()

	bi := winio.FileBasicInfo{}
	bi.FileAttributes = syscall.FILE_ATTRIBUTE_READONLY
	err = winio.SetFileBasicInfo(f, &bi)
	if err != nil {
		t.Fatal(err)
	}
	f.Close()

	err = RemoveRelative("foo", root)
	if err != nil {
		t.Fatal(err)
	}
}
