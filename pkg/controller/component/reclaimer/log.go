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

package reclaimer

import (
	"github.com/go-logr/logr"

	"github.com/vesoft-inc/nebula-operator/pkg/logging"
)

// Please don't use directly, but use getLog.
// Examples:
//   log := getLog().WithName("name").WithValues("key", "value")
//   log.Info(...)
var _log = logging.Log.WithName("controller").WithName("reclaimer")

func getLog() logr.Logger { return _log }
