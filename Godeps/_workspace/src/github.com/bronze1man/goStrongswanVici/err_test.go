package goStrongswanVici

import (
	"fmt"
	"testing"
)

func TestHandlePanic(ot *testing.T) {
	err := handlePanic(func() error {
		panic("1")
	})
	if err == nil {
		panic("err==nil")
	}
	if err.Error() != "1" {
		panic(`err.Error()!="1"`)
	}

	err = handlePanic(func() error {
		return fmt.Errorf("%d", 2)
	})
	if err == nil {
		panic("err==nil")
	}
	if err.Error() != "2" {
		panic(`err.Error()!="2" ` + err.Error())
	}
}
