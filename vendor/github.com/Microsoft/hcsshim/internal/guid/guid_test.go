package guid

import (
	"encoding/json"
	"fmt"
	"testing"
)

func Test_New(t *testing.T) {
	g := New()
	g2 := New()
	if g == g2 {
		t.Fatal("GUID's should not be equal when generated")
	}
}

func Test_FromString(t *testing.T) {
	g := New()
	g2 := FromString(g.String())
	if g != g2 {
		t.Fatalf("GUID's not equal %v, %v", g, g2)
	}
}

func Test_MarshalJSON(t *testing.T) {
	g := New()
	gs := g.String()
	js, err := json.Marshal(g)
	if err != nil {
		t.Fatalf("failed to marshal with %v", err)
	}
	gsJSON := fmt.Sprintf("\"%s\"", gs)
	if gsJSON != string(js) {
		t.Fatalf("failed to marshal %s != %s", gsJSON, string(js))
	}
}

func Test_MarshalJSON_Ptr(t *testing.T) {
	g := New()
	gs := g.String()
	js, err := json.Marshal(&g)
	if err != nil {
		t.Fatalf("failed to marshal with %v", err)
	}
	gsJSON := fmt.Sprintf("\"%s\"", gs)
	if gsJSON != string(js) {
		t.Fatalf("failed to marshal %s != %s", gsJSON, string(js))
	}
}

func Test_MarshalJSON_Nested(t *testing.T) {
	type test struct {
		G GUID
	}
	t1 := test{
		G: New(),
	}
	gs := t1.G.String()
	js, err := json.Marshal(t1)
	if err != nil {
		t.Fatalf("failed to marshal with %v", err)
	}
	gsJSON := fmt.Sprintf("{\"G\":\"%s\"}", gs)
	if gsJSON != string(js) {
		t.Fatalf("failed to marshal %s != %s", gsJSON, string(js))
	}
}

func Test_MarshalJSON_Nested_Ptr(t *testing.T) {
	type test struct {
		G *GUID
	}
	v := New()
	t1 := test{
		G: &v,
	}
	gs := t1.G.String()
	js, err := json.Marshal(t1)
	if err != nil {
		t.Fatalf("failed to marshal with %v", err)
	}
	gsJSON := fmt.Sprintf("{\"G\":\"%s\"}", gs)
	if gsJSON != string(js) {
		t.Fatalf("failed to marshal %s != %s", gsJSON, string(js))
	}
}

func Test_UnmarshalJSON(t *testing.T) {
	g := New()
	js, _ := json.Marshal(g)
	var g2 GUID
	err := json.Unmarshal(js, &g2)
	if err != nil {
		t.Fatalf("failed to unmarshal with: %v", err)
	}
	if g != g2 {
		t.Fatalf("failed to unmarshal %s != %s", g, g2)
	}
}

func Test_UnmarshalJSON_Nested(t *testing.T) {
	type test struct {
		G GUID
	}
	t1 := test{
		G: New(),
	}
	js, _ := json.Marshal(t1)
	var t2 test
	err := json.Unmarshal(js, &t2)
	if err != nil {
		t.Fatalf("failed to unmarshal with: %v", err)
	}
	if t1.G != t2.G {
		t.Fatalf("failed to unmarshal %v != %v", t1.G, t2.G)
	}
}

func Test_UnmarshalJSON_Nested_Ptr(t *testing.T) {
	type test struct {
		G *GUID
	}
	v := New()
	t1 := test{
		G: &v,
	}
	js, _ := json.Marshal(t1)
	var t2 test
	err := json.Unmarshal(js, &t2)
	if err != nil {
		t.Fatalf("failed to unmarshal with: %v", err)
	}
	if *t1.G != *t2.G {
		t.Fatalf("failed to unmarshal %v != %v", t1.G, t2.G)
	}
}
