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

package logging

import (
	"github.com/go-logr/logr"
	ctrl "sigs.k8s.io/controller-runtime"
	"sigs.k8s.io/controller-runtime/pkg/log"
	runtimezap "sigs.k8s.io/controller-runtime/pkg/log/zap"
)

type (
	Logger  = logr.Logger
	Options = runtimezap.Options
	Opts    = runtimezap.Opts
)

var (
	Log        = log.Log
	LoggerFrom = ctrl.LoggerFrom
	LoggerInto = ctrl.LoggerInto
	SetLogger  = ctrl.SetLogger

	New             = runtimezap.New
	NewRaw          = runtimezap.NewRaw
	Level           = runtimezap.Level
	StacktraceLevel = runtimezap.StacktraceLevel
	RawZapOpts      = runtimezap.RawZapOpts
	UseDevMode      = runtimezap.UseDevMode
	UseFlagOptions  = runtimezap.UseFlagOptions
)
