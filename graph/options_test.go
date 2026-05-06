package graph

import (
	"strconv"
	"testing"
)

type namedInt int

func TestOptionsIntKey(t *testing.T) {
	t.Run("default", func(t *testing.T) {
		got, err := (Options{}).IntKey("missing", 12)
		if err != nil {
			t.Fatal(err)
		}
		if got != 12 {
			t.Fatalf("expected default 12, got %d", got)
		}
	})

	for _, c := range []struct {
		name string
		val  any
		want int
	}{
		{name: "int", val: int(-1), want: -1},
		{name: "int8", val: int8(-2), want: -2},
		{name: "int16", val: int16(-3), want: -3},
		{name: "int32", val: int32(-4), want: -4},
		{name: "int64", val: int64(-5), want: -5},
		{name: "uint", val: uint(1), want: 1},
		{name: "uint8", val: uint8(2), want: 2},
		{name: "uint16", val: uint16(3), want: 3},
		{name: "uint32", val: uint32(4), want: 4},
		{name: "uint64", val: uint64(5), want: 5},
	} {
		t.Run(c.name, func(t *testing.T) {
			got, err := Options{"value": c.val}.IntKey("value", 99)
			if err != nil {
				t.Fatal(err)
			}
			if got != c.want {
				t.Fatalf("expected %d, got %d", c.want, got)
			}
		})
	}

	for _, c := range []struct {
		name string
		val  any
	}{
		{name: "float32", val: float32(1)},
		{name: "float64", val: float64(1)},
		{name: "string", val: "1"},
		{name: "bool", val: true},
		{name: "named int", val: namedInt(1)},
		{name: "uint64 overflow", val: uint64(maxInt) + 1},
	} {
		t.Run(c.name, func(t *testing.T) {
			got, err := Options{"value": c.val}.IntKey("value", 99)
			if err == nil {
				t.Fatalf("expected error, got nil and value %d", got)
			}
			if got != 99 {
				t.Fatalf("expected default 99 on error, got %d", got)
			}
		})
	}

	if strconv.IntSize == 32 {
		got, err := Options{"value": int64(maxInt) + 1}.IntKey("value", 99)
		if err == nil {
			t.Fatalf("expected int64 overflow error, got nil and value %d", got)
		}
		if got != 99 {
			t.Fatalf("expected default 99 on int64 overflow, got %d", got)
		}
	}
}
