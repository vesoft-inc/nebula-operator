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

package profile

import (
	"net/http"
	"net/http/pprof"
	"os"

	"github.com/spf13/pflag"
	"k8s.io/klog/v2"
)

type Options struct {
	// EnableProfile is the flag about whether to enable pprof profiling.
	EnableProfile bool
	// ProfilingBindAddress is the TCP address for pprof profiling.
	// Defaults to :6060 if unspecified.
	ProfilingBindAddress string
}

func (o *Options) AddFlags(fs *pflag.FlagSet) {
	fs.BoolVar(&o.EnableProfile, "enable-pprof", false, "Enable profiling via web interface host:port/debug/pprof/.")
	fs.StringVar(&o.ProfilingBindAddress, "profiling-bind-address", ":6060", "The TCP address for serving profiling(e.g. 127.0.0.1:6060, :6060).")
}

func installProfiling(mux *http.ServeMux) {
	mux.HandleFunc("/debug/pprof/", pprof.Index)
	mux.HandleFunc("/debug/pprof/cmdline", pprof.Cmdline)
	mux.HandleFunc("/debug/pprof/profile", pprof.Profile)
	mux.HandleFunc("/debug/pprof/symbol", pprof.Symbol)
	mux.HandleFunc("/debug/pprof/trace", pprof.Trace)
}

func ListenAndServe(opts Options) {
	if opts.EnableProfile {
		mux := http.NewServeMux()
		installProfiling(mux)
		klog.Infof("Starting profiling on port %s", opts.ProfilingBindAddress)
		go func() {
			httpServer := http.Server{
				Addr:    opts.ProfilingBindAddress,
				Handler: mux,
			}
			if err := httpServer.ListenAndServe(); err != nil {
				klog.Errorf("failed to enable profiling: %v", err)
				os.Exit(1)
			}
		}()
	}
}
