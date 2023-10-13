package e2evalidator

import (
	"fmt"
	"reflect"
	"strings"

	"github.com/go-playground/validator/v10"
)

type Ruler interface {
	Cmp(val any) error
}

type Rule string

func (r Rule) Cmp(val any) error {
	return validator.New().Var(val, string(r))
}

type AnyStruct struct {
	Val any
}

func (a AnyStruct) Cmp(val any) error {
	if !reflect.DeepEqual(val, a.Val) {
		return fmt.Errorf("objects not equal, expected %v, got %v", a, val)
	}
	return nil
}

func StructWithRules(actual any, rulesMapping map[string]Ruler) (errMessages []string) {
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

			if err := rule.Cmp(val); err != nil {
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
