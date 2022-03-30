/*
Copyright 2021 Vesoft Inc.

Licensed under the Apache License, Version 2.0 (the "License");
you may not use this file except in compliance with the License.
You may obtain a copy of the License at

    http://www.apache.org/licenses/LICENSE-2.0

Unless required by applicable law or agreed to in writing, software
distributed under the License is distributed on an "AS IS" BASIS,
WITHOUT WARRANTIES OR CONDITIONS OF ANY KIND, either express or implied.
See the License for the specific language governing permissions and
limitations under the License.
*/

package retry

import (
	"context"
	"time"

	"k8s.io/apimachinery/pkg/util/wait"
)

type DoneFunc func(ctx context.Context) (bool, error)

func UntilTimeout(ctx context.Context, interval, timeout time.Duration, fn DoneFunc) error {
	cx, cancel := context.WithTimeout(ctx, timeout)
	defer cancel()
	return Until(cx, interval, fn)
}

func Until(ctx context.Context, interval time.Duration, fn DoneFunc) error {
	var stop <-chan struct{}
	if ctx != nil {
		stop = ctx.Done()
	}
	return wait.PollImmediateUntil(interval, func() (bool, error) {
		done, err := fn(ctx)
		if err != nil {
			return false, err
		}
		return done, nil
	}, stop)
}
