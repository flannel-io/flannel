package ip

// Taken from a patch by David Anderson who submitted it
// but got rejected by the golang team

import (
	"encoding/binary"
	"unsafe"
)

// NativeEndian is the ByteOrder of the current system.
var NativeEndian binary.ByteOrder

func init() {
	// Examine the memory layout of an int16 to determine system
	// endianness.
	var one int16 = 1
	b := (*byte)(unsafe.Pointer(&one))
	if *b == 0 {
		NativeEndian = binary.BigEndian
	} else {
		NativeEndian = binary.LittleEndian
	}
}

func NativelyLittle() bool {
	return NativeEndian == binary.LittleEndian
}
