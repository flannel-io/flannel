package goStrongswanVici

import (
	"bytes"
	"encoding/json"
	"fmt"
	"reflect"
	"testing"
)

func TestMsg(ot *testing.T) {
	for _, msg := range []map[string]interface{}{
		map[string]interface{}{
			"a": "1",
		},
		map[string]interface{}{
			"a": []string{
				"1", "2",
			},
		},
		map[string]interface{}{
			"a": map[string]interface{}{
				"d": "e",
				"e": []string{
					"1", "2",
				},
			},
		},
		map[string]interface{}{
			"a": []string{
				"1", "2",
			},
			"b": "a",
			"c": map[string]interface{}{
				"d": "e",
				"e": []string{
					"1", "2",
				},
			},
		},
		map[string]interface{}{
			"key1": "value1",
			"section1": map[string]interface{}{
				"sub-section": map[string]interface{}{
					"key2": "value2",
				},
				"list1": []string{"item1", "item2"},
			},
		},
	} {
		buf := &bytes.Buffer{}
		in := segment{
			typ:  stCMD_REQUEST,
			name: "good",
			msg:  msg,
		}
		err := writeSegment(buf, in)
		mustNotError(err)
		content := buf.Bytes()
		out, err := readSegment(buf)
		mustNotError(err)
		//fmt.Println(content)
		if !reflect.DeepEqual(in, out) {
			in1, err := json.Marshal(in.msg)
			mustNotError(err)
			out1, err := json.Marshal(out.msg)
			mustNotError(err)
			fmt.Println(content)
			panic("!reflect.DeepEqual(in,out)\n" + string(in1) + "\n" + string(out1))
		}
	}

	content := []byte{
		0x0, 0x0, 0x0, 0x5e, //length 94
		0x1,                                     //CMD_RESPONSE
		0x3,                                     //KEY_VALUE
		0x6, 0x64, 0x61, 0x65, 0x6d, 0x6f, 0x6e, //daemon
		0x0, 0x6, 0x63, 0x68, 0x61, 0x72, 0x6f, 0x6e, //charon
		0x3, 0x7, 0x76, 0x65, 0x72, 0x73, 0x69, 0x6f, 0x6e, 0x0, 0x5, 0x35, 0x2e, 0x32, 0x2e, 0x32,
		0x3, 0x7, 0x73, 0x79, 0x73, 0x6e, 0x61, 0x6d, 0x65, 0x0, 0x5, 0x4c, 0x69, 0x6e, 0x75, 0x78, 0x3, 0x7, 0x72,
		0x65, 0x6c, 0x65, 0x61, 0x73, 0x65, 0x0, 0x11, 0x33, 0x2e, 0x31, 0x33, 0x2e, 0x30, 0x2d, 0x34, 0x34, 0x2d,
		0x67, 0x65, 0x6e, 0x65, 0x72, 0x69, 0x63, 0x3, 0x7, 0x6d, 0x61, 0x63, 0x68, 0x69, 0x6e, 0x65, 0x0, 0x6, 0x78,
		0x38, 0x36, 0x5f, 0x36, 0x34}
	buf := bytes.NewBuffer(content)
	out, err := readSegment(buf)
	mustNotError(err)

	in := segment{
		typ: stCMD_RESPONSE,
		msg: map[string]interface{}{
			"daemon":  "charon",
			"machine": "x86_64",
			"release": "3.13.0-44-generic",
			"sysname": "Linux",
			"version": "5.2.2",
		},
	}
	if !reflect.DeepEqual(in, out) {
		in1, err := json.Marshal(in.msg)
		mustNotError(err)
		out1, err := json.Marshal(out.msg)
		mustNotError(err)
		panic("!reflect.DeepEqual(in,out)\n" + string(in1) + "\n" + string(out1))
	}
}
func mustNotError(err error) {
	if err != nil {
		panic(err)
	}
}
