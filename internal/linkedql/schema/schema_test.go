package schema

import (
	"encoding/json"
	"testing"
)

func TestMarshalSchema(t *testing.T) {
	out := Generate()
	var o any
	err := json.Unmarshal(out, &o)
	if err != nil {
		t.Fatal(err)
	}
}
