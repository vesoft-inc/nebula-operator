package e2evalidator

import (
	"fmt"
	"reflect"
	"strings"

	"github.com/go-playground/validator/v10"
)

type Rule string

func StructWithRules(actual any, rulesMapping map[string]Rule) (errMessages []string) {
	for field, rule := range rulesMapping {
		actualVal := reflect.ValueOf(actual)

		isValid := true
		for _, name := range strings.Split(field, ".") {
			if actualVal.Kind() == reflect.Ptr {
				actualVal = actualVal.Elem()
			}
			actualVal = actualVal.FieldByName(name)
			if !actualVal.IsValid() {
				isValid = false
				errMessages = append(errMessages, fmt.Sprintf("field %q is invalid at %q\n", field, name))
				break
			}
		}

		if isValid {
			if actualVal.Kind() == reflect.Ptr {
				actualVal = actualVal.Elem()
			}
			val := actualVal.Interface()
			if err := validator.New().Var(val, string(rule)); err != nil {
				errMessages = append(errMessages, fmt.Sprintf("field %q(%v) does not match %q\n", field, val, rule))
			}
		}
	}

	return errMessages
}

func RuleAnd(rs ...Rule) Rule {
	ss := make([]string, len(rs))
	for i, r := range rs {
		ss[i] = string(r)
	}
	return Rule(strings.Join(ss, ","))
}

func Eq(v any) Rule {
	return Rule(fmt.Sprintf("eq=%v", v))
}

func Ne(v any) Rule {
	return Rule(fmt.Sprintf("ne=%v", v))
}
