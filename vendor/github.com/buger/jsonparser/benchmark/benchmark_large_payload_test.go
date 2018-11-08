/*
   Each test should process 24kb json record (based on Discourse API)
   It should read 2 arrays, and for each item in array get few fields.
   Basically it means processing full JSON file.
*/
package benchmark

import (
	"github.com/buger/jsonparser"
	"testing"
	// "github.com/Jeffail/gabs"
	// "github.com/bitly/go-simplejson"
	"encoding/json"
	"github.com/a8m/djson"
	jlexer "github.com/mailru/easyjson/jlexer"
	"github.com/pquerna/ffjson/ffjson"
	// "github.com/antonholmquist/jason"
	// "fmt"
)

/*
   github.com/buger/jsonparser
*/
func BenchmarkJsonParserLarge(b *testing.B) {
	for i := 0; i < b.N; i++ {
		jsonparser.ArrayEach(largeFixture, func(value []byte, dataType jsonparser.ValueType, offset int, err error) {
			jsonparser.Get(value, "username")
			nothing()
		}, "users")

		jsonparser.ArrayEach(largeFixture, func(value []byte, dataType jsonparser.ValueType, offset int, err error) {
			jsonparser.GetInt(value, "id")
			jsonparser.Get(value, "slug")
			nothing()
		}, "topics", "topics")
	}
}

/*
   encoding/json
*/
func BenchmarkEncodingJsonStructLarge(b *testing.B) {
	for i := 0; i < b.N; i++ {
		var data LargePayload
		json.Unmarshal(largeFixture, &data)

		for _, u := range data.Users {
			nothing(u.Username)
		}

		for _, t := range data.Topics.Topics {
			nothing(t.Id, t.Slug)
		}
	}
}

func BenchmarkEncodingJsonInterfaceLarge(b *testing.B) {
	for i := 0; i < b.N; i++ {
		var data interface{}
		json.Unmarshal(largeFixture, &data)
		m := data.(map[string]interface{})

		users := m["users"].([]interface{})
		for _, u := range users {
			nothing(u.(map[string]interface{})["username"].(string))
		}

		topics := m["topics"].(map[string]interface{})["topics"].([]interface{})
		for _, t := range topics {
			tI := t.(map[string]interface{})
			nothing(tI["id"].(float64), tI["slug"].(string))
		}
	}
}

/*
   github.com/pquerna/ffjson
*/

func BenchmarkFFJsonLarge(b *testing.B) {
	for i := 0; i < b.N; i++ {
		var data LargePayload
		ffjson.Unmarshal(largeFixture, &data)

		for _, u := range data.Users {
			nothing(u.Username)
		}

		for _, t := range data.Topics.Topics {
			nothing(t.Id, t.Slug)
		}
	}
}

/*
   github.com/mailru/easyjson
*/
func BenchmarkEasyJsonLarge(b *testing.B) {
	for i := 0; i < b.N; i++ {
		lexer := &jlexer.Lexer{Data: largeFixture}
		data := new(LargePayload)
		data.UnmarshalEasyJSON(lexer)

		for _, u := range data.Users {
			nothing(u.Username)
		}

		for _, t := range data.Topics.Topics {
			nothing(t.Id, t.Slug)
		}
	}
}

/*
   github.com/a8m/djson
*/
func BenchmarkDjsonLarge(b *testing.B) {
	for i := 0; i < b.N; i++ {
		m, _ := djson.DecodeObject(largeFixture)
		users := m["users"].([]interface{})
		for _, u := range users {
			nothing(u.(map[string]interface{})["username"].(string))
		}

		topics := m["topics"].(map[string]interface{})["topics"].([]interface{})
		for _, t := range topics {
			tI := t.(map[string]interface{})
			nothing(tI["id"].(float64), tI["slug"].(string))
		}
	}
}
