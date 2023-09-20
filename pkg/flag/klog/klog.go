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

package klogflag

import (
	"flag"
	"os"

	"github.com/spf13/pflag"
	"k8s.io/klog/v2"
)

func Add(fs *pflag.FlagSet) {
	flagSet := flag.NewFlagSet(os.Args[0], flag.ExitOnError)
	klog.InitFlags(flagSet)
	fs.AddGoFlagSet(flagSet)
}
