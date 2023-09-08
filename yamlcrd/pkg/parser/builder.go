package parser

import (
	"errors"
	"fmt"
)

type builder struct {
	errors []error
}

func (b *builder) Errorf(format string, args ...any) {
	err := fmt.Errorf(format, args...)
	b.errors = append(b.errors, err)
}

func (b *builder) Err() error {
	if len(b.errors) == 0 {
		return nil
	}
	return errors.Join(b.errors...)
}
