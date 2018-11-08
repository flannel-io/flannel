package jsonparser

import (
	"fmt"
	"testing"
)

var testPaths = [][]string{
	[]string{"test"},
	[]string{"these"},
	[]string{"keys"},
	[]string{"please"},
}

func testIter(data []byte) (err error) {
	EachKey(data, func(idx int, value []byte, vt ValueType, iterErr error) {
		if iterErr != nil {
			err = fmt.Errorf("Error parsing json: %s", iterErr.Error())
		}
	}, testPaths...)
	return err
}

func TestPanickingErrors(t *testing.T) {
	if err := testIter([]byte(`{"test":`)); err == nil {
		t.Error("Expected error...")
	}

	if err := testIter([]byte(`{"test":0}some":[{"these":[{"keys":"some"}]}]}some"}]}],"please":"some"}`)); err == nil {
		t.Error("Expected error...")
	}

	if _, _, _, err := Get([]byte(`{"test":`), "test"); err == nil {
		t.Error("Expected error...")
	}

	if _, _, _, err := Get([]byte(`{"some":0}some":[{"some":[{"some":"some"}]}]}some"}]}],"some":"some"}`), "x"); err == nil {
		t.Error("Expected error...")
	}
}
