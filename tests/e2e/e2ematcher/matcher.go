package e2ematcher

import (
	"fmt"

	"github.com/hashicorp/go-multierror"
)

var _ Matcher = MatcherFunc(nil)

type (
	Matcher interface {
		Match(val any) error
	}

	MatcherFunc func(val any) error
)

func (f MatcherFunc) Match(val any) error {
	return f(val)
}

func And(ms ...Matcher) Matcher {
	return MatcherFunc(func(val any) error {
		for _, m := range ms {
			if err := m.Match(val); err != nil {
				return err
			}
		}
		return nil
	})
}

func Any(ms ...Matcher) Matcher {
	return MatcherFunc(func(val any) error {
		var retErr error
		for _, m := range ms {
			if err := m.Match(val); err != nil {
				retErr = multierror.Append(retErr, err)
			} else {
				return nil
			}
		}
		return retErr
	})
}

func MapContainsMatchers[K comparable, V any](m map[K]V) map[string]any {
	matchers := make(map[string]any, len(m))
	for k, v := range m {
		matchers[fmt.Sprint(k)] = ValidatorEq(v)
	}
	return matchers
}
