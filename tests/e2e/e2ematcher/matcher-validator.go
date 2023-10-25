package e2ematcher

import (
	"fmt"
	"strings"

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
	return ValidatorMatcher(fmt.Sprintf("eq=%v", validatorReplaceValue(v)))
}

func ValidatorNe(v any) Matcher {
	return ValidatorMatcher(fmt.Sprintf("ne=%v", validatorReplaceValue(v)))
}

func validatorReplaceValue(v any) any {
	switch vv := v.(type) {
	case string:
		if strings.ContainsAny(vv, ",|") {
			vv = strings.ReplaceAll(vv, ",", "0x2C")
			vv = strings.ReplaceAll(vv, "|", "0x7C")
			return vv
		}
	}
	return v
}
