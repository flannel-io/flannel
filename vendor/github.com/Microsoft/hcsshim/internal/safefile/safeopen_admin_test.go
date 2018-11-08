// +build admin

package safefile

import (
	"os"
	"path/filepath"
	"syscall"
	"testing"
)

func TestOpenRelative(t *testing.T) {
	badroot, err := tempRoot()
	if err != nil {
		t.Fatal(err)
	}
	defer os.RemoveAll(badroot.Name())
	defer badroot.Close()

	root, err := tempRoot()
	if err != nil {
		t.Fatal(err)
	}
	defer os.RemoveAll(root.Name())
	defer root.Close()

	// Create a file
	f, err := OpenRelative("foo", root, 0, syscall.FILE_SHARE_READ, FILE_CREATE, 0)
	if err != nil {
		t.Fatal(err)
	}
	f.Close()

	// Create a directory
	err = MkdirRelative("dir", root)
	if err != nil {
		t.Fatal(err)
	}

	// Create a file in the bad root
	f, err = os.Create(filepath.Join(badroot.Name(), "badfile"))
	if err != nil {
		t.Fatal(err)
	}
	f.Close()

	// Create a directory symlink to the bad root
	err = os.Symlink(badroot.Name(), filepath.Join(root.Name(), "dsymlink"))
	if err != nil {
		t.Fatal(err)
	}

	// Create a file symlink to the bad file
	err = os.Symlink(filepath.Join(badroot.Name(), "badfile"), filepath.Join(root.Name(), "symlink"))
	if err != nil {
		t.Fatal(err)
	}

	// Make sure opens cannot happen through the symlink
	f, err = OpenRelative("dsymlink/foo", root, 0, syscall.FILE_SHARE_READ, FILE_CREATE, 0)
	if err == nil {
		f.Close()
		t.Fatal("created file in wrong tree!")
	}
	t.Log(err)

	// Check again using EnsureNotReparsePointRelative
	err = EnsureNotReparsePointRelative("dsymlink", root)
	if err == nil {
		t.Fatal("reparse check should have failed")
	}
	t.Log(err)

	// Make sure links work
	err = LinkRelative("foo", root, "hardlink", root)
	if err != nil {
		t.Fatal(err)
	}

	// Even inside directories
	err = LinkRelative("foo", root, "dir/bar", root)
	if err != nil {
		t.Fatal(err)
	}

	// Make sure links cannot happen through the symlink
	err = LinkRelative("foo", root, "dsymlink/hardlink", root)
	if err == nil {
		f.Close()
		t.Fatal("created link in wrong tree!")
	}
	t.Log(err)

	// In either direction
	err = LinkRelative("dsymlink/badfile", root, "bar", root)
	if err == nil {
		f.Close()
		t.Fatal("created link in wrong tree!")
	}
	t.Log(err)

	// Make sure remove cannot happen through the symlink
	err = RemoveRelative("symlink/badfile", root)
	if err == nil {
		t.Fatal("remove in wrong tree!")
	}

	// Remove the symlink
	err = RemoveAllRelative("symlink", root)
	if err != nil {
		t.Fatal(err)
	}

	// Make sure it's not possible to escape with .. (NT doesn't support .. at the kernel level)
	f, err = OpenRelative("..", root, syscall.GENERIC_READ, syscall.FILE_SHARE_READ, FILE_OPEN, 0)
	if err == nil {
		t.Fatal("escaped the directory")
	}
	t.Log(err)

	// Should not have touched the other directory
	if _, err = os.Lstat(filepath.Join(badroot.Name(), "badfile")); err != nil {
		t.Fatal(err)
	}
}
