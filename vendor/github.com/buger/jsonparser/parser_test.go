package jsonparser

import (
	"bytes"
	"fmt"
	_ "fmt"
	"reflect"
	"testing"
)

// Set it to non-empty value if want to run only specific test
var activeTest = ""

func toArray(data []byte) (result [][]byte) {
	ArrayEach(data, func(value []byte, dataType ValueType, offset int, err error) {
		result = append(result, value)
	})

	return
}

func toStringArray(data []byte) (result []string) {
	ArrayEach(data, func(value []byte, dataType ValueType, offset int, err error) {
		result = append(result, string(value))
	})

	return
}

type GetTest struct {
	desc string
	json string
	path []string

	isErr   bool
	isFound bool

	data interface{}
}

type SetTest struct {
	desc    string
	json    string
	setData string
	path    []string

	isErr   bool
	isFound bool

	data interface{}
}

type DeleteTest struct {
	desc string
	json string
	path []string

	data interface{}
}

var deleteTests = []DeleteTest{
	{
		desc: "Delete test key",
		json: `{"test":"input"}`,
		path: []string{"test"},
		data: `{}`,
	},
	{
		desc: "Delete object",
		json: `{"test":"input"}`,
		path: []string{},
		data: ``,
	},
	{
		desc: "Delete a nested object",
		json: `{"test":"input","new.field":{"key": "new object"}}`,
		path: []string{"new.field", "key"},
		data: `{"test":"input","new.field":{}}`,
	},
	{
		desc: "Deleting a key that doesn't exist should return the same object",
		json: `{"test":"input"}`,
		path: []string{"test2"},
		data: `{"test":"input"}`,
	},
	{
		desc: "Delete object in an array",
		json: `{"test":[{"key":"val-obj1"}]}`,
		path: []string{"test", "[0]"},
		data: `{"test":[]}`,
	},
	{
		desc: "Deleting a object in an array that doesn't exists should return the same object",
		json: `{"test":[{"key":"val-obj1"}]}`,
		path: []string{"test", "[1]"},
		data: `{"test":[{"key":"val-obj1"}]}`,
	},
	{
		desc: "Delete a complex object in a nested array",
		json: `{"test":[{"key":[{"innerKey":"innerKeyValue"}]}]}`,
		path: []string{"test", "[0]", "key", "[0]"},
		data: `{"test":[{"key":[]}]}`,
	},
	{
		desc: "Delete known key (simple type within nested array)",
		json: `{"test":[{"key":["innerKey"]}]}`,
		path: []string{"test", "[0]", "key", "[0]"},
		data: `{"test":[{"key":[]}]}`,
	},
	{
		desc: "Delete in empty json",
		json: `{}`,
		path: []string{},
		data: ``,
	},
	{
		desc: "Delete empty array",
		json: `[]`,
		path: []string{},
		data: ``,
	},
	{
		desc: "Deleting non json should return the same value",
		json: `1.323`,
		path: []string{"foo"},
		data: `1.323`,
	},
	{
		desc: "Delete known key (top level array)",
		json: `[{"key":"val-obj1"}]`,
		path: []string{"[0]"},
		data: `[]`,
	},
	{ // This test deletes the key instead of returning a parse error, as checking for the malformed JSON would reduce performance (this is not ideal)
		desc: `malformed with trailing whitespace`,
		json: `{"a":1 `,
		path: []string{"a"},
		data: `{ `,
	},
	{ // This test dels the key instead of returning a parse error, as checking for the malformed JSON would reduce performance (this is not ideal)
		desc: "malformed 'colon chain', delete b",
		json: `{"a":"b":"c"}`,
		path: []string{"b"},
		data: `{"a":}`,
	},
	{
		desc: "Delete object without inner array",
		json: `{"a": {"b": 1}, "b": 2}`,
		path: []string{"b"},
		data: `{"a": {"b": 1}}`,
	},
	{
		desc: "Delete object without inner array",
		json: `{"a": [{"b": 1}], "b": 2}`,
		path: []string{"b"},
		data: `{"a": [{"b": 1}]}`,
	},
	{
		desc: "Delete object without inner array",
		json: `{"a": {"c": {"b": 3}, "b": 1}, "b": 2}`,
		path: []string{"a", "b"},
		data: `{"a": {"c": {"b": 3}}, "b": 2}`,
	},
	{
		desc: "Delete object without inner array",
		json: `{"a": [{"c": {"b": 3}, "b": 1}], "b": 2}`,
		path: []string{"a", "[0]", "b"},
		data: `{"a": [{"c": {"b": 3}}], "b": 2}`,
	},
	{
		desc: "Remove trailing comma if last object is deleted",
		json: `{"a": "1", "b": "2"}`,
		path: []string{"b"},
		data: `{"a": "1"}`,
	},
	{
		desc: "Correctly delete first element with space-comma",
		json: `{"a": "1" ,"b": "2" }`,
		path: []string{"a"},
		data: `{"b": "2" }`,
	},
	{
		desc: "Correctly delete middle element with space-comma",
		json: `{"a": "1" ,"b": "2" , "c": 3}`,
		path: []string{"b"},
		data: `{"a": "1" , "c": 3}`,
	},
}

var setTests = []SetTest{
	{
		desc:    "set unknown key (string)",
		json:    `{"test":"input"}`,
		isFound: true,
		path:    []string{"new.field"},
		setData: `"new value"`,
		data:    `{"test":"input","new.field":"new value"}`,
	},
	{
		desc:    "set known key (string)",
		json:    `{"test":"input"}`,
		isFound: true,
		path:    []string{"test"},
		setData: `"new value"`,
		data:    `{"test":"new value"}`,
	},
	{
		desc:    "set unknown key (object)",
		json:    `{"test":"input"}`,
		isFound: true,
		path:    []string{"new.field"},
		setData: `{"key": "new object"}`,
		data:    `{"test":"input","new.field":{"key": "new object"}}`,
	},
	{
		desc:    "set known key (object)",
		json:    `{"test":"input"}`,
		isFound: true,
		path:    []string{"test"},
		setData: `{"key": "new object"}`,
		data:    `{"test":{"key": "new object"}}`,
	},
	{
		desc:    "set known key (object within array)",
		json:    `{"test":[{"key":"val-obj1"}]}`,
		isFound: true,
		path:    []string{"test", "[0]"},
		setData: `{"key":"new object"}`,
		data:    `{"test":[{"key":"new object"}]}`,
	},
	{
		desc:    "set unknown key (replace object)",
		json:    `{"test":[{"key":"val-obj1"}]}`,
		isFound: true,
		path:    []string{"test", "newKey"},
		setData: `"new object"`,
		data:    `{"test":{"newKey":"new object"}}`,
	},
	{
		desc:    "set unknown key (complex object within nested array)",
		json:    `{"test":[{"key":[{"innerKey":"innerKeyValue"}]}]}`,
		isFound: true,
		path:    []string{"test", "[0]", "key", "[0]", "newInnerKey"},
		setData: `{"key":"new object"}`,
		data:    `{"test":[{"key":[{"innerKey":"innerKeyValue","newInnerKey":{"key":"new object"}}]}]}`,
	},
	{
		desc:    "set known key (complex object within nested array)",
		json:    `{"test":[{"key":[{"innerKey":"innerKeyValue"}]}]}`,
		isFound: true,
		path:    []string{"test", "[0]", "key", "[0]", "innerKey"},
		setData: `{"key":"new object"}`,
		data:    `{"test":[{"key":[{"innerKey":{"key":"new object"}}]}]}`,
	},
	{
		desc:    "set unknown key (object, partial subtree exists)",
		json:    `{"test":{"input":"output"}}`,
		isFound: true,
		path:    []string{"test", "new.field"},
		setData: `{"key":"new object"}`,
		data:    `{"test":{"input":"output","new.field":{"key":"new object"}}}`,
	},
	{
		desc:    "set unknown key (object, empty partial subtree exists)",
		json:    `{"test":{}}`,
		isFound: true,
		path:    []string{"test", "new.field"},
		setData: `{"key":"new object"}`,
		data:    `{"test":{"new.field":{"key":"new object"}}}`,
	},
	{
		desc:    "set unknown key (object, no subtree exists)",
		json:    `{"test":"input"}`,
		isFound: true,
		path:    []string{"new.field", "nested", "value"},
		setData: `{"key": "new object"}`,
		data:    `{"test":"input","new.field":{"nested":{"value":{"key": "new object"}}}}`,
	},
	{
		desc:    "set in empty json",
		json:    `{}`,
		isFound: true,
		path:    []string{"foo"},
		setData: `"null"`,
		data:    `{"foo":"null"}`,
	},
	{
		desc:    "set subtree in empty json",
		json:    `{}`,
		isFound: true,
		path:    []string{"foo", "bar"},
		setData: `"null"`,
		data:    `{"foo":{"bar":"null"}}`,
	},
	{
		desc:    "set in empty string - not found",
		json:    ``,
		isFound: false,
		path:    []string{"foo"},
		setData: `"null"`,
		data:    ``,
	},
	{
		desc:    "set in Number - not found",
		json:    `1.323`,
		isFound: false,
		path:    []string{"foo"},
		setData: `"null"`,
		data:    `1.323`,
	},
	{
		desc:    "set known key (top level array)",
		json:    `[{"key":"val-obj1"}]`,
		isFound: true,
		path:    []string{"[0]", "key"},
		setData: `"new object"`,
		data:    `[{"key":"new object"}]`,
	},
	{
		desc:    "set unknown key (trailing whitespace)",
		json:    `{"key":"val-obj1"}  `,
		isFound: true,
		path:    []string{"alt-key"},
		setData: `"new object"`,
		data:    `{"key":"val-obj1","alt-key":"new object"}  `,
	},
	{ // This test sets the key instead of returning a parse error, as checking for the malformed JSON would reduce performance (this is not ideal)
		desc:    `malformed with trailing whitespace`,
		json:    `{"a":1 `,
		path:    []string{"a"},
		setData: `2`,
		isFound: true,
		data:    `{"a":2 `,
	},
	{ // This test sets the key instead of returning a parse error, as checking for the malformed JSON would reduce performance (this is not ideal)
		desc:    "malformed 'colon chain', set second string",
		json:    `{"a":"b":"c"}`,
		path:    []string{"b"},
		setData: `"d"`,
		isFound: true,
		data:    `{"a":"b":"d"}`,
	},
	{
		desc:    "set indexed path to object on empty JSON",
		json:    `{}`,
		path:    []string{"top", "[0]", "middle", "[0]", "bottom"},
		setData: `"value"`,
		isFound: true,
		data:    `{"top":[{"middle":[{"bottom":"value"}]}]}`,
	},
	{
		desc:    "set indexed path on existing object with object",
		json:    `{"top":[{"middle":[]}]}`,
		path:    []string{"top", "[0]", "middle", "[0]", "bottom"},
		setData: `"value"`,
		isFound: true,
		data:    `{"top":[{"middle":[{"bottom":"value"}]}]}`,
	},
	{
		desc:    "set indexed path on existing object with value",
		json:    `{"top":[{"middle":[]}]}`,
		path:    []string{"top", "[0]", "middle", "[0]"},
		setData: `"value"`,
		isFound: true,
		data:    `{"top":[{"middle":["value"]}]}`,
	},
	{
		desc:    "set indexed path on empty object with value",
		json:    `{}`,
		path:    []string{"top", "[0]", "middle", "[0]"},
		setData: `"value"`,
		isFound: true,
		data:    `{"top":[{"middle":["value"]}]}`,
	},
	{
		desc:    "set indexed path on object with existing array",
		json:    `{"top":["one", "two", "three"]}`,
		path:    []string{"top", "[2]"},
		setData: `"value"`,
		isFound: true,
		data:    `{"top":["one", "two", "value"]}`,
	},
}

var getTests = []GetTest{
	// Trivial tests
	{
		desc:    "read string",
		json:    `""`,
		isFound: true,
		data:    ``,
	},
	{
		desc:    "read number",
		json:    `0`,
		isFound: true,
		data:    `0`,
	},
	{
		desc:    "read object",
		json:    `{}`,
		isFound: true,
		data:    `{}`,
	},
	{
		desc:    "read array",
		json:    `[]`,
		isFound: true,
		data:    `[]`,
	},
	{
		desc:    "read boolean",
		json:    `true`,
		isFound: true,
		data:    `true`,
	},

	// Found key tests
	{
		desc:    "handling multiple nested keys with same name",
		json:    `{"a":[{"b":1},{"b":2},3],"c":{"c":[1,2]}} }`,
		path:    []string{"c", "c"},
		isFound: true,
		data:    `[1,2]`,
	},
	{
		desc:    "read basic key",
		json:    `{"a":"b"}`,
		path:    []string{"a"},
		isFound: true,
		data:    `b`,
	},
	{
		desc:    "read basic key with space",
		json:    `{"a": "b"}`,
		path:    []string{"a"},
		isFound: true,
		data:    `b`,
	},
	{
		desc:    "read composite key",
		json:    `{"a": { "b":{"c":"d" }}}`,
		path:    []string{"a", "b", "c"},
		isFound: true,
		data:    `d`,
	},
	{
		desc:    `read numberic value as string`,
		json:    `{"a": "b", "c": 1}`,
		path:    []string{"c"},
		isFound: true,
		data:    `1`,
	},
	{
		desc:    `handle multiple nested keys with same name`,
		json:    `{"a":[{"b":1},{"b":2},3],"c":{"c":[1,2]}} }`,
		path:    []string{"c", "c"},
		isFound: true,
		data:    `[1,2]`,
	},
	{
		desc:    `read string values with quotes`,
		json:    `{"a": "string\"with\"quotes"}`,
		path:    []string{"a"},
		isFound: true,
		data:    `string\"with\"quotes`,
	},
	{
		desc:    `read object`,
		json:    `{"a": { "b":{"c":"d" }}}`,
		path:    []string{"a", "b"},
		isFound: true,
		data:    `{"c":"d" }`,
	},
	{
		desc:    `empty path`,
		json:    `{"c":"d" }`,
		path:    []string{},
		isFound: true,
		data:    `{"c":"d" }`,
	},
	{
		desc:    `formatted JSON value`,
		json:    "{\n  \"a\": \"b\"\n}",
		path:    []string{"a"},
		isFound: true,
		data:    `b`,
	},
	{
		desc:    `formatted JSON value 2`,
		json:    "{\n  \"a\":\n    {\n\"b\":\n   {\"c\":\"d\",\n\"e\": \"f\"}\n}\n}",
		path:    []string{"a", "b"},
		isFound: true,
		data:    "{\"c\":\"d\",\n\"e\": \"f\"}",
	},
	{
		desc:    `whitespace`,
		json:    " \n\r\t{ \n\r\t\"whitespace\" \n\r\t: \n\r\t333 \n\r\t} \n\r\t",
		path:    []string{"whitespace"},
		isFound: true,
		data:    "333",
	},
	{
		desc:    `escaped backslash quote`,
		json:    `{"a": "\\\""}`,
		path:    []string{"a"},
		isFound: true,
		data:    `\\\"`,
	},
	{
		desc:    `unescaped backslash quote`,
		json:    `{"a": "\\"}`,
		path:    []string{"a"},
		isFound: true,
		data:    `\\`,
	},
	{
		desc:    `unicode in JSON`,
		json:    `{"a": "15°C"}`,
		path:    []string{"a"},
		isFound: true,
		data:    `15°C`,
	},
	{
		desc:    `no padding + nested`,
		json:    `{"a":{"a":"1"},"b":2}`,
		path:    []string{"b"},
		isFound: true,
		data:    `2`,
	},
	{
		desc:    `no padding + nested + array`,
		json:    `{"a":{"b":[1,2]},"c":3}`,
		path:    []string{"c"},
		isFound: true,
		data:    `3`,
	},
	{
		desc:    `empty key`,
		json:    `{"":{"":{"":true}}}`,
		path:    []string{"", "", ""},
		isFound: true,
		data:    `true`,
	},

	// Escaped key tests
	{
		desc:    `key with simple escape`,
		json:    `{"a\\b":1}`,
		path:    []string{"a\\b"},
		isFound: true,
		data:    `1`,
	},
	{
		desc:    `key and value with whitespace escapes`,
		json:    `{"key\b\f\n\r\tkey":"value\b\f\n\r\tvalue"}`,
		path:    []string{"key\b\f\n\r\tkey"},
		isFound: true,
		data:    `value\b\f\n\r\tvalue`, // value is not unescaped since this is Get(), but the key should work correctly
	},
	{
		desc:    `key with Unicode escape`,
		json:    `{"a\u00B0b":1}`,
		path:    []string{"a\u00B0b"},
		isFound: true,
		data:    `1`,
	},
	{
		desc:    `key with complex escape`,
		json:    `{"a\uD83D\uDE03b":1}`,
		path:    []string{"a\U0001F603b"},
		isFound: true,
		data:    `1`,
	},

	{ // This test returns a match instead of a parse error, as checking for the malformed JSON would reduce performance
		desc:    `malformed with trailing whitespace`,
		json:    `{"a":1 `,
		path:    []string{"a"},
		isFound: true,
		data:    `1`,
	},
	{ // This test returns a match instead of a parse error, as checking for the malformed JSON would reduce performance
		desc:    `malformed with wrong closing bracket`,
		json:    `{"a":1]`,
		path:    []string{"a"},
		isFound: true,
		data:    `1`,
	},

	// Not found key tests
	{
		desc:    `empty input`,
		json:    ``,
		path:    []string{"a"},
		isFound: false,
	},
	{
		desc:    "non-existent key 1",
		json:    `{"a":"b"}`,
		path:    []string{"c"},
		isFound: false,
	},
	{
		desc:    "non-existent key 2",
		json:    `{"a":"b"}`,
		path:    []string{"b"},
		isFound: false,
	},
	{
		desc:    "non-existent key 3",
		json:    `{"aa":"b"}`,
		path:    []string{"a"},
		isFound: false,
	},
	{
		desc:    "apply scope of parent when search for nested key",
		json:    `{"a": { "b": 1}, "c": 2 }`,
		path:    []string{"a", "b", "c"},
		isFound: false,
	},
	{
		desc:    `apply scope to key level`,
		json:    `{"a": { "b": 1}, "c": 2 }`,
		path:    []string{"b"},
		isFound: false,
	},
	{
		desc:    `handle escaped quote in key name in JSON`,
		json:    `{"key\"key": 1}`,
		path:    []string{"key"},
		isFound: false,
	},
	{
		desc:    "handling multiple keys with different name",
		json:    `{"a":{"a":1},"b":{"a":3,"c":[1,2]}}`,
		path:    []string{"a", "c"},
		isFound: false,
	},
	{
		desc:    "handling nested json",
		json:    `{"a":{"b":{"c":1},"d":4}}`,
		path:    []string{"a", "d"},
		isFound: true,
		data:    `4`,
	},

	// Error/invalid tests
	{
		desc:    `handle escaped quote in key name in JSON`,
		json:    `{"key\"key": 1}`,
		path:    []string{"key"},
		isFound: false,
	},
	{
		desc:    `missing closing brace, but can still find key`,
		json:    `{"a":"b"`,
		path:    []string{"a"},
		isFound: true,
		data:    `b`,
	},
	{
		desc:  `missing value closing quote`,
		json:  `{"a":"b`,
		path:  []string{"a"},
		isErr: true,
	},
	{
		desc:  `missing value closing curly brace`,
		json:  `{"a": { "b": "c"`,
		path:  []string{"a"},
		isErr: true,
	},
	{
		desc:  `missing value closing square bracket`,
		json:  `{"a": [1, 2, 3 }`,
		path:  []string{"a"},
		isErr: true,
	},
	{
		desc:  `missing value 1`,
		json:  `{"a":`,
		path:  []string{"a"},
		isErr: true,
	},
	{
		desc:  `missing value 2`,
		json:  `{"a": `,
		path:  []string{"a"},
		isErr: true,
	},
	{
		desc:  `missing value 3`,
		json:  `{"a":}`,
		path:  []string{"a"},
		isErr: true,
	},
	{
		desc:    `malformed array (no closing brace)`,
		json:    `{"a":[, "b":123}`,
		path:    []string{"b"},
		isFound: false,
	},
	{ // Issue #81
		desc:    `missing key in object in array`,
		json:    `{"p":{"a":[{"u":"abc","t":"th"}]}}`,
		path:    []string{"p", "a", "[0]", "x"},
		isFound: false,
	},
	{ // Issue #81 counter test
		desc:    `existing key in object in array`,
		json:    `{"p":{"a":[{"u":"abc","t":"th"}]}}`,
		path:    []string{"p", "a", "[0]", "u"},
		isFound: true,
		data:    "abc",
	},
	{ // This test returns not found instead of a parse error, as checking for the malformed JSON would reduce performance
		desc:    "malformed key (followed by comma followed by colon)",
		json:    `{"a",:1}`,
		path:    []string{"a"},
		isFound: false,
	},
	{ // This test returns a match instead of a parse error, as checking for the malformed JSON would reduce performance (this is not ideal)
		desc:    "malformed 'colon chain', lookup first string",
		json:    `{"a":"b":"c"}`,
		path:    []string{"a"},
		isFound: true,
		data:    "b",
	},
	{ // This test returns a match instead of a parse error, as checking for the malformed JSON would reduce performance (this is not ideal)
		desc:    "malformed 'colon chain', lookup second string",
		json:    `{"a":"b":"c"}`,
		path:    []string{"b"},
		isFound: true,
		data:    "c",
	},

	// Array index paths
	{
		desc:    "last key in path is index",
		json:    `{"a":[{"b":1},{"b":"2"}, 3],"c":{"c":[1,2]}}`,
		path:    []string{"a", "[1]"},
		isFound: true,
		data:    `{"b":"2"}`,
	},
	{
		desc:    "get string from array",
		json:    `{"a":[{"b":1},"foo", 3],"c":{"c":[1,2]}}`,
		path:    []string{"a", "[1]"},
		isFound: true,
		data:    "foo",
	},
	{
		desc:    "key in path is index",
		json:    `{"a":[{"b":"1"},{"b":"2"},3],"c":{"c":[1,2]}}`,
		path:    []string{"a", "[0]", "b"},
		isFound: true,
		data:    `1`,
	},
	{
		desc: "last key in path is an index to value in array (formatted json)",
		json: `{
		    "a": [
			{
			    "b": 1
			},
			{"b":"2"},
			3
		    ],
		    "c": {
			"c": [
			    1,
			    2
			]
		    }
		}`,
		path:    []string{"a", "[1]"},
		isFound: true,
		data:    `{"b":"2"}`,
	},
	{
		desc: "key in path is index (formatted json)",
		json: `{
		    "a": [
			{"b": 1},
			{"b": "2"},
			3
		    ],
		    "c": {
			"c": [
			    1,
			    2
			]
		    }
		}`,
		path:    []string{"a", "[0]", "b"},
		isFound: true,
		data:    `1`,
	},
}

var getIntTests = []GetTest{
	{
		desc:    `read numeric value as number`,
		json:    `{"a": "b", "c": 1}`,
		path:    []string{"c"},
		isFound: true,
		data:    int64(1),
	},
	{
		desc:    `read numeric value as number in formatted JSON`,
		json:    "{\"a\": \"b\", \"c\": 1 \n}",
		path:    []string{"c"},
		isFound: true,
		data:    int64(1),
	},
}

var getFloatTests = []GetTest{
	{
		desc:    `read numeric value as number`,
		json:    `{"a": "b", "c": 1.123}`,
		path:    []string{"c"},
		isFound: true,
		data:    float64(1.123),
	},
	{
		desc:    `read numeric value as number in formatted JSON`,
		json:    "{\"a\": \"b\", \"c\": 23.41323 \n}",
		path:    []string{"c"},
		isFound: true,
		data:    float64(23.41323),
	},
}

var getStringTests = []GetTest{
	{
		desc:    `Translate Unicode symbols`,
		json:    `{"c": "test"}`,
		path:    []string{"c"},
		isFound: true,
		data:    `test`,
	},
	{
		desc:    `Translate Unicode symbols`,
		json:    `{"c": "15\u00b0C"}`,
		path:    []string{"c"},
		isFound: true,
		data:    `15°C`,
	},
	{
		desc:    `Translate supplementary Unicode symbols`,
		json:    `{"c": "\uD83D\uDE03"}`, // Smiley face (UTF16 surrogate pair)
		path:    []string{"c"},
		isFound: true,
		data:    "\U0001F603", // Smiley face
	},
	{
		desc:    `Translate escape symbols`,
		json:    `{"c": "\\\""}`,
		path:    []string{"c"},
		isFound: true,
		data:    `\"`,
	},
	{
		desc:    `key and value with whitespace escapes`,
		json:    `{"key\b\f\n\r\tkey":"value\b\f\n\r\tvalue"}`,
		path:    []string{"key\b\f\n\r\tkey"},
		isFound: true,
		data:    "value\b\f\n\r\tvalue", // value is unescaped since this is GetString()
	},
}

var getBoolTests = []GetTest{
	{
		desc:    `read boolean true as boolean`,
		json:    `{"a": "b", "c": true}`,
		path:    []string{"c"},
		isFound: true,
		data:    true,
	},
	{
		desc:    `boolean true in formatted JSON`,
		json:    "{\"a\": \"b\", \"c\": true \n}",
		path:    []string{"c"},
		isFound: true,
		data:    true,
	},
	{
		desc:    `read boolean false as boolean`,
		json:    `{"a": "b", "c": false}`,
		path:    []string{"c"},
		isFound: true,
		data:    false,
	},
	{
		desc:    `boolean true in formatted JSON`,
		json:    "{\"a\": \"b\", \"c\": false \n}",
		path:    []string{"c"},
		isFound: true,
		data:    false,
	},
	{
		desc:  `read fake boolean true`,
		json:  `{"a": txyz}`,
		path:  []string{"a"},
		isErr: true,
	},
	{
		desc:  `read fake boolean false`,
		json:  `{"a": fwxyz}`,
		path:  []string{"a"},
		isErr: true,
	},
	{
		desc:    `read boolean true with whitespace and another key`,
		json:    "{\r\t\n \"a\"\r\t\n :\r\t\n true\r\t\n ,\r\t\n \"b\": 1}",
		path:    []string{"a"},
		isFound: true,
		data:    true,
	},
}

var getArrayTests = []GetTest{
	{
		desc:    `read array of simple values`,
		json:    `{"a": { "b":[1,2,3,4]}}`,
		path:    []string{"a", "b"},
		isFound: true,
		data:    []string{`1`, `2`, `3`, `4`},
	},
	{
		desc:    `read array via empty path`,
		json:    `[1,2,3,4]`,
		path:    []string{},
		isFound: true,
		data:    []string{`1`, `2`, `3`, `4`},
	},
	{
		desc:    `read array of objects`,
		json:    `{"a": { "b":[{"x":1},{"x":2},{"x":3},{"x":4}]}}`,
		path:    []string{"a", "b"},
		isFound: true,
		data:    []string{`{"x":1}`, `{"x":2}`, `{"x":3}`, `{"x":4}`},
	},
	{
		desc:    `read nested array`,
		json:    `{"a": [[[1]],[[2]]]}`,
		path:    []string{"a"},
		isFound: true,
		data:    []string{`[[1]]`, `[[2]]`},
	},
}

// checkFoundAndNoError checks the dataType and error return from Get*() against the test case expectations.
// Returns true the test should proceed to checking the actual data returned from Get*(), or false if the test is finished.
func getTestCheckFoundAndNoError(t *testing.T, testKind string, test GetTest, jtype ValueType, value interface{}, err error) bool {
	isFound := (err != KeyPathNotFoundError)
	isErr := (err != nil && err != KeyPathNotFoundError)

	if test.isErr != isErr {
		// If the call didn't match the error expectation, fail
		t.Errorf("%s test '%s' isErr mismatch: expected %t, obtained %t (err %v). Value: %v", testKind, test.desc, test.isErr, isErr, err, value)
		return false
	} else if isErr {
		// Else, if there was an error, don't fail and don't check isFound or the value
		return false
	} else if test.isFound != isFound {
		// Else, if the call didn't match the is-found expectation, fail
		t.Errorf("%s test '%s' isFound mismatch: expected %t, obtained %t", testKind, test.desc, test.isFound, isFound)
		return false
	} else if !isFound {
		// Else, if no value was found, don't fail and don't check the value
		return false
	} else {
		// Else, there was no error and a value was found, so check the value
		return true
	}
}

func runGetTests(t *testing.T, testKind string, tests []GetTest, runner func(GetTest) (interface{}, ValueType, error), resultChecker func(GetTest, interface{}) (bool, interface{})) {
	for _, test := range tests {
		if activeTest != "" && test.desc != activeTest {
			continue
		}

		fmt.Println("Running:", test.desc)

		value, dataType, err := runner(test)

		if getTestCheckFoundAndNoError(t, testKind, test, dataType, value, err) {
			if test.data == nil {
				t.Errorf("MALFORMED TEST: %v", test)
				continue
			}

			if ok, expected := resultChecker(test, value); !ok {
				if expectedBytes, ok := expected.([]byte); ok {
					expected = string(expectedBytes)
				}
				if valueBytes, ok := value.([]byte); ok {
					value = string(valueBytes)
				}
				t.Errorf("%s test '%s' expected to return value %v, but did returned %v instead", testKind, test.desc, expected, value)
			}
		}
	}
}

func setTestCheckFoundAndNoError(t *testing.T, testKind string, test SetTest, value interface{}, err error) bool {
	isFound := (err != KeyPathNotFoundError)
	isErr := (err != nil && err != KeyPathNotFoundError)

	if test.isErr != isErr {
		// If the call didn't match the error expectation, fail
		t.Errorf("%s test '%s' isErr mismatch: expected %t, obtained %t (err %v). Value: %v", testKind, test.desc, test.isErr, isErr, err, value)
		return false
	} else if isErr {
		// Else, if there was an error, don't fail and don't check isFound or the value
		return false
	} else if test.isFound != isFound {
		// Else, if the call didn't match the is-found expectation, fail
		t.Errorf("%s test '%s' isFound mismatch: expected %t, obtained %t", testKind, test.desc, test.isFound, isFound)
		return false
	} else if !isFound {
		// Else, if no value was found, don't fail and don't check the value
		return false
	} else {
		// Else, there was no error and a value was found, so check the value
		return true
	}
}

func runSetTests(t *testing.T, testKind string, tests []SetTest, runner func(SetTest) (interface{}, ValueType, error), resultChecker func(SetTest, interface{}) (bool, interface{})) {
	for _, test := range tests {
		if activeTest != "" && test.desc != activeTest {
			continue
		}

		fmt.Println("Running:", test.desc)

		value, _, err := runner(test)

		if setTestCheckFoundAndNoError(t, testKind, test, value, err) {
			if test.data == nil {
				t.Errorf("MALFORMED TEST: %v", test)
				continue
			}

			if string(value.([]byte)) != test.data {
				t.Errorf("Unexpected result on %s test '%s'", testKind, test.desc)
				t.Log("Got:     ", string(value.([]byte)))
				t.Log("Expected:", test.data)
				t.Log("Error:   ", err)
			}
		}
	}
}

func runDeleteTests(t *testing.T, testKind string, tests []DeleteTest, runner func(DeleteTest) interface{}, resultChecker func(DeleteTest, interface{}) (bool, interface{})) {
	for _, test := range tests {
		if activeTest != "" && test.desc != activeTest {
			continue
		}

		fmt.Println("Running:", test.desc)

		value := runner(test)

		if test.data == nil {
			t.Errorf("MALFORMED TEST: %v", test)
			continue
		}

		if ok, expected := resultChecker(test, value); !ok {
			if expectedBytes, ok := expected.([]byte); ok {
				expected = string(expectedBytes)
			}
			if valueBytes, ok := value.([]byte); ok {
				value = string(valueBytes)
			}
			t.Errorf("%s test '%s' expected to return value %v, but did returned %v instead", testKind, test.desc, expected, value)
		}
	}
}

func TestSet(t *testing.T) {
	runSetTests(t, "Set()", setTests,
		func(test SetTest) (value interface{}, dataType ValueType, err error) {
			value, err = Set([]byte(test.json), []byte(test.setData), test.path...)
			return
		},
		func(test SetTest, value interface{}) (bool, interface{}) {
			expected := []byte(test.data.(string))
			return bytes.Equal(expected, value.([]byte)), expected
		},
	)
}

func TestDelete(t *testing.T) {
	runDeleteTests(t, "Delete()", deleteTests,
		func(test DeleteTest) interface{} {
			return Delete([]byte(test.json), test.path...)
		},
		func(test DeleteTest, value interface{}) (bool, interface{}) {
			expected := []byte(test.data.(string))
			return bytes.Equal(expected, value.([]byte)), expected
		},
	)
}

func TestGet(t *testing.T) {
	runGetTests(t, "Get()", getTests,
		func(test GetTest) (value interface{}, dataType ValueType, err error) {
			value, dataType, _, err = Get([]byte(test.json), test.path...)
			return
		},
		func(test GetTest, value interface{}) (bool, interface{}) {
			expected := []byte(test.data.(string))
			return bytes.Equal(expected, value.([]byte)), expected
		},
	)
}

func TestGetString(t *testing.T) {
	runGetTests(t, "GetString()", getStringTests,
		func(test GetTest) (value interface{}, dataType ValueType, err error) {
			value, err = GetString([]byte(test.json), test.path...)
			return value, String, err
		},
		func(test GetTest, value interface{}) (bool, interface{}) {
			expected := test.data.(string)
			return expected == value.(string), expected
		},
	)
}

func TestGetInt(t *testing.T) {
	runGetTests(t, "GetInt()", getIntTests,
		func(test GetTest) (value interface{}, dataType ValueType, err error) {
			value, err = GetInt([]byte(test.json), test.path...)
			return value, Number, err
		},
		func(test GetTest, value interface{}) (bool, interface{}) {
			expected := test.data.(int64)
			return expected == value.(int64), expected
		},
	)
}

func TestGetFloat(t *testing.T) {
	runGetTests(t, "GetFloat()", getFloatTests,
		func(test GetTest) (value interface{}, dataType ValueType, err error) {
			value, err = GetFloat([]byte(test.json), test.path...)
			return value, Number, err
		},
		func(test GetTest, value interface{}) (bool, interface{}) {
			expected := test.data.(float64)
			return expected == value.(float64), expected
		},
	)
}

func TestGetBoolean(t *testing.T) {
	runGetTests(t, "GetBoolean()", getBoolTests,
		func(test GetTest) (value interface{}, dataType ValueType, err error) {
			value, err = GetBoolean([]byte(test.json), test.path...)
			return value, Boolean, err
		},
		func(test GetTest, value interface{}) (bool, interface{}) {
			expected := test.data.(bool)
			return expected == value.(bool), expected
		},
	)
}

func TestGetSlice(t *testing.T) {
	runGetTests(t, "Get()-for-arrays", getArrayTests,
		func(test GetTest) (value interface{}, dataType ValueType, err error) {
			value, dataType, _, err = Get([]byte(test.json), test.path...)
			return
		},
		func(test GetTest, value interface{}) (bool, interface{}) {
			expected := test.data.([]string)
			return reflect.DeepEqual(expected, toStringArray(value.([]byte))), expected
		},
	)
}

func TestArrayEach(t *testing.T) {
	mock := []byte(`{"a": { "b":[{"x": 1} ,{"x":2},{ "x":3}, {"x":4} ]}}`)
	count := 0

	ArrayEach(mock, func(value []byte, dataType ValueType, offset int, err error) {
		count++

		switch count {
		case 1:
			if string(value) != `{"x": 1}` {
				t.Errorf("Wrong first item: %s", string(value))
			}
		case 2:
			if string(value) != `{"x":2}` {
				t.Errorf("Wrong second item: %s", string(value))
			}
		case 3:
			if string(value) != `{ "x":3}` {
				t.Errorf("Wrong third item: %s", string(value))
			}
		case 4:
			if string(value) != `{"x":4}` {
				t.Errorf("Wrong forth item: %s", string(value))
			}
		default:
			t.Errorf("Should process only 4 items")
		}
	}, "a", "b")
}

func TestArrayEachEmpty(t *testing.T) {
	funcError := func([]byte, ValueType, int, error) { t.Errorf("Run func not allow") }

	type args struct {
		data []byte
		cb   func(value []byte, dataType ValueType, offset int, err error)
		keys []string
	}
	tests := []struct {
		name       string
		args       args
		wantOffset int
		wantErr    bool
	}{
		{"Empty array", args{[]byte("[]"), funcError, []string{}}, 1, false},
		{"Empty array with space", args{[]byte("[ ]"), funcError, []string{}}, 2, false},
		{"Empty array with \n", args{[]byte("[\n]"), funcError, []string{}}, 2, false},
		{"Empty field array", args{[]byte("{\"data\": []}"), funcError, []string{"data"}}, 10, false},
		{"Empty field array with space", args{[]byte("{\"data\": [ ]}"), funcError, []string{"data"}}, 11, false},
		{"Empty field array with \n", args{[]byte("{\"data\": [\n]}"), funcError, []string{"data"}}, 11, false},
	}
	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			gotOffset, err := ArrayEach(tt.args.data, tt.args.cb, tt.args.keys...)
			if (err != nil) != tt.wantErr {
				t.Errorf("ArrayEach() error = %v, wantErr %v", err, tt.wantErr)
				return
			}
			if gotOffset != tt.wantOffset {
				t.Errorf("ArrayEach() = %v, want %v", gotOffset, tt.wantOffset)
			}
		})
	}
}

type keyValueEntry struct {
	key       string
	value     string
	valueType ValueType
}

func (kv keyValueEntry) String() string {
	return fmt.Sprintf("[%s: %s (%s)]", kv.key, kv.value, kv.valueType)
}

type ObjectEachTest struct {
	desc string
	json string

	isErr   bool
	entries []keyValueEntry
}

var objectEachTests = []ObjectEachTest{
	{
		desc:    "empty object",
		json:    `{}`,
		entries: []keyValueEntry{},
	},
	{
		desc: "single key-value object",
		json: `{"key": "value"}`,
		entries: []keyValueEntry{
			{"key", "value", String},
		},
	},
	{
		desc: "multiple key-value object with many value types",
		json: `{
		  "key1": null,
		  "key2": true,
		  "key3": 1.23,
		  "key4": "string value",
		  "key5": [1,2,3],
		  "key6": {"a":"b"}
		}`,
		entries: []keyValueEntry{
			{"key1", "null", Null},
			{"key2", "true", Boolean},
			{"key3", "1.23", Number},
			{"key4", "string value", String},
			{"key5", "[1,2,3]", Array},
			{"key6", `{"a":"b"}`, Object},
		},
	},
	{
		desc: "escaped key",
		json: `{"key\"\\\/\b\f\n\r\t\u00B0": "value"}`,
		entries: []keyValueEntry{
			{"key\"\\/\b\f\n\r\t\u00B0", "value", String},
		},
	},
	// Error cases
	{
		desc:  "no object present",
		json:  ` \t\n\r`,
		isErr: true,
	},
	{
		desc:  "unmatched braces 1",
		json:  `{`,
		isErr: true,
	},
	{
		desc:  "unmatched braces 2",
		json:  `}`,
		isErr: true,
	},
	{
		desc:  "unmatched braces 3",
		json:  `}{}`,
		isErr: true,
	},
	{
		desc:  "bad key (number)",
		json:  `{123: "value"}`,
		isErr: true,
	},
	{
		desc:  "bad key (unclosed quote)",
		json:  `{"key: 123}`,
		isErr: true,
	},
	{
		desc:  "bad value (no value)",
		json:  `{"key":}`,
		isErr: true,
	},
	{
		desc:  "bad value (bogus value)",
		json:  `{"key": notavalue}`,
		isErr: true,
	},
	{
		desc:  "bad entry (missing colon)",
		json:  `{"key" "value"}`,
		isErr: true,
	},
	{
		desc:  "bad entry (no trailing comma)",
		json:  `{"key": "value" "key2": "value2"}`,
		isErr: true,
	},
	{
		desc:  "bad entry (two commas)",
		json:  `{"key": "value",, "key2": "value2"}`,
		isErr: true,
	},
}

func TestObjectEach(t *testing.T) {
	for _, test := range objectEachTests {
		if activeTest != "" && test.desc != activeTest {
			continue
		}

		// Execute ObjectEach and capture all of the entries visited, in order
		var entries []keyValueEntry
		err := ObjectEach([]byte(test.json), func(key, value []byte, valueType ValueType, off int) error {
			entries = append(entries, keyValueEntry{
				key:       string(key),
				value:     string(value),
				valueType: valueType,
			})
			return nil
		})

		// Check the correctness of the result
		isErr := (err != nil)
		if test.isErr != isErr {
			// If the call didn't match the error expectation, fail
			t.Errorf("ObjectEach test '%s' isErr mismatch: expected %t, obtained %t (err %v)", test.desc, test.isErr, isErr, err)
		} else if isErr {
			// Else, if there was an expected error, don't fail and don't check anything further
		} else if len(test.entries) != len(entries) {
			t.Errorf("ObjectEach test '%s' mismatch in number of key-value entries: expected %d, obtained %d (entries found: %s)", test.desc, len(test.entries), len(entries), entries)
		} else {
			for i, entry := range entries {
				expectedEntry := test.entries[i]
				if expectedEntry.key != entry.key {
					t.Errorf("ObjectEach test '%s' key mismatch at entry %d: expected %s, obtained %s", test.desc, i, expectedEntry.key, entry.key)
					break
				} else if expectedEntry.value != entry.value {
					t.Errorf("ObjectEach test '%s' value mismatch at entry %d: expected %s, obtained %s", test.desc, i, expectedEntry.value, entry.value)
					break
				} else if expectedEntry.valueType != entry.valueType {
					t.Errorf("ObjectEach test '%s' value type mismatch at entry %d: expected %s, obtained %s", test.desc, i, expectedEntry.valueType, entry.valueType)
					break
				} else {
					// Success for this entry
				}
			}
		}
	}
}

var testJson = []byte(`{"name": "Name", "order": "Order", "sum": 100, "len": 12, "isPaid": true, "nested": {"a":"test", "b":2, "nested3":{"a":"test3","b":4}, "c": "unknown"}, "nested2": {"a":"test2", "b":3}, "arr": [{"a":"zxc", "b": 1}, {"a":"123", "b":2}], "arrInt": [1,2,3,4], "intPtr": 10}`)

func TestEachKey(t *testing.T) {
	paths := [][]string{
		{"name"},
		{"order"},
		{"nested", "a"},
		{"nested", "b"},
		{"nested2", "a"},
		{"nested", "nested3", "b"},
		{"arr", "[1]", "b"},
		{"arrInt", "[3]"},
		{"arrInt", "[5]"}, // Should not find last key
	}

	keysFound := 0

	EachKey(testJson, func(idx int, value []byte, vt ValueType, err error) {
		keysFound++

		switch idx {
		case 0:
			if string(value) != "Name" {
				t.Error("Should find 1 key", string(value))
			}
		case 1:
			if string(value) != "Order" {
				t.Errorf("Should find 2 key")
			}
		case 2:
			if string(value) != "test" {
				t.Errorf("Should find 3 key")
			}
		case 3:
			if string(value) != "2" {
				t.Errorf("Should find 4 key")
			}
		case 4:
			if string(value) != "test2" {
				t.Error("Should find 5 key", string(value))
			}
		case 5:
			if string(value) != "4" {
				t.Errorf("Should find 6 key")
			}
		case 6:
			if string(value) != "2" {
				t.Errorf("Should find 7 key")
			}
		case 7:
			if string(value) != "4" {
				t.Error("Should find 8 key", string(value))
			}
		default:
			t.Errorf("Should found only 8 keys")
		}
	}, paths...)

	if keysFound != 8 {
		t.Errorf("Should find 8 keys: %d", keysFound)
	}
}

type ParseTest struct {
	in     string
	intype ValueType
	out    interface{}
	isErr  bool
}

var parseBoolTests = []ParseTest{
	{
		in:     "true",
		intype: Boolean,
		out:    true,
	},
	{
		in:     "false",
		intype: Boolean,
		out:    false,
	},
	{
		in:     "foo",
		intype: Boolean,
		isErr:  true,
	},
	{
		in:     "trux",
		intype: Boolean,
		isErr:  true,
	},
	{
		in:     "truex",
		intype: Boolean,
		isErr:  true,
	},
	{
		in:     "",
		intype: Boolean,
		isErr:  true,
	},
}

var parseFloatTest = []ParseTest{
	{
		in:     "0",
		intype: Number,
		out:    float64(0),
	},
	{
		in:     "0.0",
		intype: Number,
		out:    float64(0.0),
	},
	{
		in:     "1",
		intype: Number,
		out:    float64(1),
	},
	{
		in:     "1.234",
		intype: Number,
		out:    float64(1.234),
	},
	{
		in:     "1.234e5",
		intype: Number,
		out:    float64(1.234e5),
	},
	{
		in:     "-1.234e5",
		intype: Number,
		out:    float64(-1.234e5),
	},
	{
		in:     "+1.234e5", // Note: + sign not allowed under RFC7159, but our parser accepts it since it uses strconv.ParseFloat
		intype: Number,
		out:    float64(1.234e5),
	},
	{
		in:     "1.2.3",
		intype: Number,
		isErr:  true,
	},
	{
		in:     "1..1",
		intype: Number,
		isErr:  true,
	},
	{
		in:     "1a",
		intype: Number,
		isErr:  true,
	},
	{
		in:     "",
		intype: Number,
		isErr:  true,
	},
}

// parseTestCheckNoError checks the error return from Parse*() against the test case expectations.
// Returns true the test should proceed to checking the actual data returned from Parse*(), or false if the test is finished.
func parseTestCheckNoError(t *testing.T, testKind string, test ParseTest, value interface{}, err error) bool {
	if isErr := (err != nil); test.isErr != isErr {
		// If the call didn't match the error expectation, fail
		t.Errorf("%s test '%s' isErr mismatch: expected %t, obtained %t (err %v). Obtained value: %v", testKind, test.in, test.isErr, isErr, err, value)
		return false
	} else if isErr {
		// Else, if there was an error, don't fail and don't check isFound or the value
		return false
	} else {
		// Else, there was no error and a value was found, so check the value
		return true
	}
}

func runParseTests(t *testing.T, testKind string, tests []ParseTest, runner func(ParseTest) (interface{}, error), resultChecker func(ParseTest, interface{}) (bool, interface{})) {
	for _, test := range tests {
		value, err := runner(test)

		if parseTestCheckNoError(t, testKind, test, value, err) {
			if test.out == nil {
				t.Errorf("MALFORMED TEST: %v", test)
				continue
			}

			if ok, expected := resultChecker(test, value); !ok {
				if expectedBytes, ok := expected.([]byte); ok {
					expected = string(expectedBytes)
				}
				if valueBytes, ok := value.([]byte); ok {
					value = string(valueBytes)
				}
				t.Errorf("%s test '%s' expected to return value %v, but did returned %v instead", testKind, test.in, expected, value)
			}
		}
	}
}

func TestParseBoolean(t *testing.T) {
	runParseTests(t, "ParseBoolean()", parseBoolTests,
		func(test ParseTest) (value interface{}, err error) {
			return ParseBoolean([]byte(test.in))
		},
		func(test ParseTest, obtained interface{}) (bool, interface{}) {
			expected := test.out.(bool)
			return obtained.(bool) == expected, expected
		},
	)
}

func TestParseFloat(t *testing.T) {
	runParseTests(t, "ParseFloat()", parseFloatTest,
		func(test ParseTest) (value interface{}, err error) {
			return ParseFloat([]byte(test.in))
		},
		func(test ParseTest, obtained interface{}) (bool, interface{}) {
			expected := test.out.(float64)
			return obtained.(float64) == expected, expected
		},
	)
}

var parseStringTest = []ParseTest{
	{
		in:     `\uFF11`,
		intype: String,
		out:    "\uFF11",
	},
	{
		in:     `\uFFFF`,
		intype: String,
		out:    "\uFFFF",
	},
	{
		in:     `\uDF00`,
		intype: String,
		isErr:  true,
	},
}

func TestParseString(t *testing.T) {
	runParseTests(t, "ParseString()", parseStringTest,
		func(test ParseTest) (value interface{}, err error) {
			return ParseString([]byte(test.in))
		},
		func(test ParseTest, obtained interface{}) (bool, interface{}) {
			expected := test.out.(string)
			return obtained.(string) == expected, expected
		},
	)
}
