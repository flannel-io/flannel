// +build !appengine,!appenginevm

package jsonparser

import (
	"reflect"
	"strings"
	"testing"
	"unsafe"
)

var (
	// short string/[]byte sequences, as the difference between these
	// three methods is a constant overhead
	benchmarkString = "0123456789x"
	benchmarkBytes  = []byte("0123456789y")
)

func bytesEqualStrSafe(abytes []byte, bstr string) bool {
	return bstr == string(abytes)
}

func bytesEqualStrUnsafeSlower(abytes *[]byte, bstr string) bool {
	aslicehdr := (*reflect.SliceHeader)(unsafe.Pointer(abytes))
	astrhdr := reflect.StringHeader{Data: aslicehdr.Data, Len: aslicehdr.Len}
	return *(*string)(unsafe.Pointer(&astrhdr)) == bstr
}

func TestEqual(t *testing.T) {
	if !equalStr(&[]byte{}, "") {
		t.Errorf(`equalStr("", ""): expected true, obtained false`)
		return
	}

	longstr := strings.Repeat("a", 1000)
	for i := 0; i < len(longstr); i++ {
		s1, s2 := longstr[:i]+"1", longstr[:i]+"2"
		b1 := []byte(s1)

		if !equalStr(&b1, s1) {
			t.Errorf(`equalStr("a"*%d + "1", "a"*%d + "1"): expected true, obtained false`, i, i)
			break
		}
		if equalStr(&b1, s2) {
			t.Errorf(`equalStr("a"*%d + "1", "a"*%d + "2"): expected false, obtained true`, i, i)
			break
		}
	}
}

func BenchmarkEqualStr(b *testing.B) {
	for i := 0; i < b.N; i++ {
		equalStr(&benchmarkBytes, benchmarkString)
	}
}

// Alternative implementation without using unsafe
func BenchmarkBytesEqualStrSafe(b *testing.B) {
	for i := 0; i < b.N; i++ {
		bytesEqualStrSafe(benchmarkBytes, benchmarkString)
	}
}

// Alternative implementation using unsafe, but that is slower than the current implementation
func BenchmarkBytesEqualStrUnsafeSlower(b *testing.B) {
	for i := 0; i < b.N; i++ {
		bytesEqualStrUnsafeSlower(&benchmarkBytes, benchmarkString)
	}
}
