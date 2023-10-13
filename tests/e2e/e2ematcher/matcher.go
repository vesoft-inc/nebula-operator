package e2ematcher

import (
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
