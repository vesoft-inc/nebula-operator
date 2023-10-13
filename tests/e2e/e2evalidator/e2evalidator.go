package e2evalidator

import (
	"fmt"
	"github.com/go-playground/validator/v10"
	corev1 "k8s.io/api/core/v1"
	"reflect"
	"strings"
)

type Ruler interface {
	Cmp(val any) error
}

type Rule string

func (r Rule) Cmp(val any) error {
	return validator.New().Var(val, string(r))
}

type Resource corev1.ResourceRequirements

func (r Resource) Cmp(val any) error {
	if !reflect.DeepEqual(val, corev1.ResourceRequirements(r)) {
		return fmt.Errorf("resource not equal, expected %v, got %v", r, val)
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
