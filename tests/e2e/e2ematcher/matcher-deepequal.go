package e2ematcher

import (
	"fmt"

	"github.com/google/go-cmp/cmp"
)

func DeepEqual(v any) Matcher {
	return MatcherFunc(func(val any) error {
		if diff := cmp.Diff(val, v); diff != "" {
			return fmt.Errorf("DeepEqual failed %s", diff)
		}
		return nil
	})
}
