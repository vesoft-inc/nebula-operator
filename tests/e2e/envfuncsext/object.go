/*
Copyright 2023 Vesoft Inc.

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

package envfuncsext

import (
	"context"

	"k8s.io/klog/v2"
	"sigs.k8s.io/e2e-framework/klient/k8s"
	"sigs.k8s.io/e2e-framework/klient/k8s/resources"
	"sigs.k8s.io/e2e-framework/pkg/env"
	"sigs.k8s.io/e2e-framework/pkg/envconf"
)

type ObjectOption func(*ObjectOptions)

type ObjectOptions struct {
	CreateOptions []resources.CreateOption
	DeleteOptions []resources.DeleteOption
}

func (o *ObjectOptions) WithOptions(opts ...ObjectOption) *ObjectOptions {
	for _, opt := range opts {
		opt(o)
	}
	return o
}

func WithObjectCreateOptions(opts ...resources.CreateOption) ObjectOption {
	return func(o *ObjectOptions) {
		o.CreateOptions = append(o.CreateOptions, opts...)
	}
}

func WithObjectDeleteOptions(opts ...resources.DeleteOption) ObjectOption {
	return func(o *ObjectOptions) {
		o.DeleteOptions = append(o.DeleteOptions, opts...)
	}
}

func CreateObject(obj k8s.Object) env.Func {
	return func(ctx context.Context, cfg *envconf.Config) (context.Context, error) {
		if err := cfg.Client().Resources().Create(ctx, obj); err != nil {
			klog.ErrorS(err, "Create object failed", "obj", obj)
			return ctx, err
		}
		return ctx, nil
	}
}

func DeleteObject(obj k8s.Object) env.Func {
	return func(ctx context.Context, cfg *envconf.Config) (context.Context, error) {
		if err := cfg.Client().Resources().Delete(ctx, obj); err != nil {
			klog.ErrorS(err, "Delete object failed", "obj", obj)
			return ctx, err
		}
		return ctx, nil
	}
}
