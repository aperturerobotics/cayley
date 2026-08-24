//go:build !tinygo

package linkedql

import (
	"fmt"
	"strings"
)

func formatMultiError(errors []error) error {
	var joinedErr strings.Builder
	for i, err := range errors {
		if i > 0 {
			joinedErr.WriteString("; ")
		}
		joinedErr.WriteString(err.Error())
	}
	return fmt.Errorf("could not parse PropertyPath: %v", joinedErr.String())
}
