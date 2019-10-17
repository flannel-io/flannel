package converters

import (
	"bytes"
	"unsafe"
)

func UnsafeBytesToString(bs []byte) string {
	return *(*string)(unsafe.Pointer(&bs))
}

func UnsafeUTF16BytesToString(bs []byte) string {
	return UnsafeBytesToString(bytes.Trim(bs, "\x00"))
}
