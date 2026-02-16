package linkedql

import "strings"

import "fmt"

func formatMultiError(errors []error) error {
	var joinedErr strings.Builder
	for _, err := range errors {
		joinedErr.WriteString("; " + err.Error())
	}
	return fmt.Errorf("could not parse PropertyPath: %v", joinedErr.String())
}
