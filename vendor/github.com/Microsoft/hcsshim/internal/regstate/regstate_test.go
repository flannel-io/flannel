package regstate

import (
	"os"
	"testing"
)

var testKey = "runhcs-test-test-key"

func prepTest(t *testing.T) {
	err := RemoveAll(testKey, true)
	if err != nil && !os.IsNotExist(err) {
		t.Fatal(err)
	}
}

func TestLifetime(t *testing.T) {
	prepTest(t)
	k, err := Open(testKey, true)
	if err != nil {
		t.Fatal(err)
	}
	ids, err := k.Enumerate()
	if err != nil {
		t.Fatal(err)
	}
	if len(ids) != 0 {
		t.Fatal("wrong count", len(ids))
	}

	id := "a/b/c"
	key := "key"
	err = k.Set(id, key, 1)
	if err == nil {
		t.Fatal("expected error")
	}

	var i int
	err = k.Get(id, key, &i)
	if err == nil {
		t.Fatal("expected error")
	}

	err = k.Create(id, key, 2)
	if err != nil {
		t.Fatal(err)
	}

	ids, err = k.Enumerate()
	if err != nil {
		t.Fatal(err)
	}
	if len(ids) != 1 {
		t.Fatal("wrong count", len(ids))
	}
	if ids[0] != id {
		t.Fatal("wrong value", ids[0])
	}

	err = k.Get(id, key, &i)
	if err != nil {
		t.Fatal(err)
	}
	if i != 2 {
		t.Fatal("got wrong value", i)
	}

	err = k.Set(id, key, 3)
	if err != nil {
		t.Fatal(err)
	}
	err = k.Get(id, key, &i)
	if err != nil {
		t.Fatal(err)
	}
	if i != 3 {
		t.Fatal("got wrong value", i)
	}

	err = k.Remove(id)
	if err != nil {
		t.Fatal(err)
	}
	err = k.Remove(id)
	if err == nil {
		t.Fatal("expected error")
	}

	ids, err = k.Enumerate()
	if err != nil {
		t.Fatal(err)
	}
	if len(ids) != 0 {
		t.Fatal("wrong count", len(ids))
	}
}

func TestBool(t *testing.T) {
	prepTest(t)
	k, err := Open(testKey, true)
	if err != nil {
		t.Fatal(err)
	}
	id := "x"
	key := "y"
	err = k.Create(id, key, true)
	if err != nil {
		t.Fatal(err)
	}
	b := false
	err = k.Get(id, key, &b)
	if err != nil {
		t.Fatal(err)
	}
	if !b {
		t.Fatal("value did not marshal correctly")
	}
}

func TestInt(t *testing.T) {
	prepTest(t)
	k, err := Open(testKey, true)
	if err != nil {
		t.Fatal(err)
	}
	id := "x"
	key := "y"
	err = k.Create(id, key, 10)
	if err != nil {
		t.Fatal(err)
	}
	v := 0
	err = k.Get(id, key, &v)
	if err != nil {
		t.Fatal(err)
	}
	if v != 10 {
		t.Fatal("value did not marshal correctly")
	}
}

func TestString(t *testing.T) {
	prepTest(t)
	k, err := Open(testKey, true)
	if err != nil {
		t.Fatal(err)
	}
	id := "x"
	key := "y"
	err = k.Create(id, key, "blah")
	if err != nil {
		t.Fatal(err)
	}
	v := ""
	err = k.Get(id, key, &v)
	if err != nil {
		t.Fatal(err)
	}
	if v != "blah" {
		t.Fatal("value did not marshal correctly")
	}
}

func TestJson(t *testing.T) {
	prepTest(t)
	k, err := Open(testKey, true)
	if err != nil {
		t.Fatal(err)
	}
	id := "x"
	key := "y"
	v := struct{ X int }{5}
	err = k.Create(id, key, &v)
	if err != nil {
		t.Fatal(err)
	}
	v.X = 0
	err = k.Get(id, key, &v)
	if err != nil {
		t.Fatal(err)
	}
	if v.X != 5 {
		t.Fatal("value did not marshal correctly: ", v)
	}
}
