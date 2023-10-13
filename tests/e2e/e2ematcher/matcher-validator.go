package e2ematcher

import (
	"fmt"

	"github.com/go-playground/validator/v10"
)

var _ Matcher = ValidatorMatcher("")

type ValidatorMatcher string

func (m ValidatorMatcher) Match(val any) error {
	if err := validator.New().Var(val, string(m)); err != nil {
		return fmt.Errorf("validate(%s) failed %w", m, err)
	}
	return nil
}

func ValidatorEq(v any) Matcher {
	return ValidatorMatcher(fmt.Sprintf("eq=%v", v))
}

func ValidatorNe(v any) Matcher {
	return ValidatorMatcher(fmt.Sprintf("ne=%v", v))
}
