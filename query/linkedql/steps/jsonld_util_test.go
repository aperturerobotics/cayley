package steps

import (
	"fmt"
	"testing"

	"github.com/stretchr/testify/require"
)

var testCases = []struct {
	name     string
	source   any
	target   any
	expected error
}{
	{
		name:     "Single matching IDs",
		source:   map[string]any{"@id": "a"},
		target:   map[string]any{"@id": "a"},
		expected: nil,
	},
	{
		name:     "Single non matching IDs",
		source:   map[string]any{"@id": "a"},
		target:   map[string]any{"@id": "b"},
		expected: fmt.Errorf("expected \"a\" but instead received \"b\""),
	},
	{
		name:     "Single matching properties",
		source:   map[string]any{"http://example.com/name": "Alice"},
		target:   map[string]any{"http://example.com/name": "Alice"},
		expected: nil,
	},
	{
		name:     "Single non matching properties",
		source:   map[string]any{"http://example.com/name": "Alice"},
		target:   map[string]any{"http://example.com/name": "Bob"},
		expected: fmt.Errorf("expected \"Alice\" but instead received \"Bob\""),
	},
	{
		name:     "Single matching property with multiple values ordered",
		source:   map[string]any{"http://example.com/name": []any{"Alice", "Bob"}},
		target:   map[string]any{"http://example.com/name": []any{"Alice", "Bob"}},
		expected: nil,
	},
	{
		name:     "Single matching property with multiple values unordered",
		source:   map[string]any{"http://example.com/name": []any{"Alice", "Bob"}},
		target:   map[string]any{"http://example.com/name": []any{"Bob", "Alice"}},
		expected: nil,
	},
	{
		name:     "Single non matching property with multiple values",
		source:   map[string]any{"http://example.com/name": []any{"Alice", "Bob"}},
		target:   map[string]any{"http://example.com/name": []any{"Dan", "Alice"}},
		expected: fmt.Errorf("no matching values for the item \"Bob\" in []interface {}{\"Dan\", \"Alice\"}"),
	},
	{
		name:     "Single non matching property with multiple values non matching length",
		source:   map[string]any{"http://example.com/name": []any{"Alice", "Bob"}},
		target:   map[string]any{"http://example.com/name": []any{"Alice"}},
		expected: fmt.Errorf("expected multiple values but instead received the single value: \"Alice\""),
	},
	{
		name: "Single matching nested",
		source: map[string]any{
			"http://example.com/friend": map[string]any{
				"@id": "alice",
			},
		},
		target: map[string]any{
			"http://example.com/friend": map[string]any{
				"@id": "alice",
			},
		},
		expected: nil,
	},
	{
		name: "Single non matching nested",
		source: map[string]any{
			"http://example.com/friend": map[string]any{
				"@id": "alice",
			},
		},
		target: map[string]any{
			"http://example.com/friend": map[string]any{
				"@id": "bob",
			},
		},
		expected: fmt.Errorf("expected \"alice\" but instead received \"bob\""),
	},
	{
		name:     "Single matching properties with @value string",
		source:   map[string]any{"http://example.com/name": map[string]any{"@value": "Alice"}},
		target:   map[string]any{"http://example.com/name": map[string]any{"@value": "Alice"}},
		expected: nil,
	},
	{
		name:     "Single non matching properties with @value string",
		source:   map[string]any{"http://example.com/name": map[string]any{"@value": "Alice"}},
		target:   map[string]any{"http://example.com/name": map[string]any{"@value": "Bob"}},
		expected: fmt.Errorf("expected \"Alice\" but instead received \"Bob\""),
	},
	{
		name:     "Single matching properties with @value string and string",
		source:   map[string]any{"http://example.com/name": map[string]any{"@value": "Alice"}},
		target:   map[string]any{"http://example.com/name": "Alice"},
		expected: nil,
	},
	{
		name:     "Single matching properties with string and @value string",
		source:   map[string]any{"http://example.com/name": "Alice"},
		target:   map[string]any{"http://example.com/name": map[string]any{"@value": "Alice"}},
		expected: nil,
	},
	{
		name:     "Single matching properties with @value string array string",
		source:   map[string]any{"http://example.com/name": []any{map[string]any{"@value": "Alice"}}},
		target:   map[string]any{"http://example.com/name": "Alice"},
		expected: nil,
	},
	{
		name:     "Single matching properties with string and @value string array",
		source:   map[string]any{"http://example.com/name": "Alice"},
		target:   map[string]any{"http://example.com/name": []any{map[string]any{"@value": "Alice"}}},
		expected: nil,
	},
}

func TestIsomorphic(t *testing.T) {
	for _, c := range testCases {
		t.Run(c.name, func(t *testing.T) {
			require.Equal(t, c.expected, isomorphic(c.source, c.target))
		})
	}
}
