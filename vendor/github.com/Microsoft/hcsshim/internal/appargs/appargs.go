// Package appargs provides argument validation routines for use with
// github.com/urfave/cli.
package appargs

import (
	"errors"
	"strconv"

	"github.com/urfave/cli"
)

// Validator is an argument validator function. It returns the number of
// arguments consumed or -1 on error.
type Validator = func([]string) int

// String is a validator for strings.
func String(args []string) int {
	if len(args) == 0 {
		return -1
	}
	return 1
}

// NonEmptyString is a validator for non-empty strings.
func NonEmptyString(args []string) int {
	if len(args) == 0 || args[0] == "" {
		return -1
	}
	return 1
}

// Int returns a validator for integers.
func Int(base int, min int, max int) Validator {
	return func(args []string) int {
		if len(args) == 0 {
			return -1
		}
		i, err := strconv.ParseInt(args[0], base, 0)
		if err != nil || int(i) < min || int(i) > max {
			return -1
		}
		return 1
	}
}

// Optional returns a validator that treats an argument as optional.
func Optional(v Validator) Validator {
	return func(args []string) int {
		if len(args) == 0 {
			return 0
		}
		return v(args)
	}
}

// Rest returns a validator that validates each of the remaining arguments.
func Rest(v Validator) Validator {
	return func(args []string) int {
		count := len(args)
		for len(args) != 0 {
			n := v(args)
			if n < 0 {
				return n
			}
			args = args[n:]
		}
		return count
	}
}

// ErrInvalidUsage is returned when there is a validation error.
var ErrInvalidUsage = errors.New("invalid command usage")

// Validate can be used as a command's Before function to validate the arguments
// to the command.
func Validate(vs ...Validator) cli.BeforeFunc {
	return func(context *cli.Context) error {
		remaining := context.Args()
		for _, v := range vs {
			consumed := v(remaining)
			if consumed < 0 {
				return ErrInvalidUsage
			}
			remaining = remaining[consumed:]
		}

		if len(remaining) > 0 {
			return ErrInvalidUsage
		}

		return nil
	}
}
