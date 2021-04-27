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

package version

import (
	"fmt"
	"runtime"
	"strconv"
	"time"
)

const undefined = "<undefined>"

var (
	gitVersion     = undefined
	gitCommit      = undefined
	gitTimestamp   = undefined
	buildTimestamp = undefined
)

// Info contains version information.
type Info struct {
	GitVersion string `json:"gitVersion"`
	GitCommit  string `json:"gitCommit"`
	GitDate    string `json:"gitDate"`
	BuildDate  string `json:"buildDate"`
	GoVersion  string `json:"goVersion"`
	Compiler   string `json:"compiler"`
	Platform   string `json:"platform"`
}

func (i *Info) String() string {
	return fmt.Sprintf("%#v", i)
}

func Version() Info {
	return Info{
		GitVersion: gitVersion,
		GitCommit:  gitCommit,
		GitDate:    convDate(gitTimestamp),
		BuildDate:  convDate(buildTimestamp),
		GoVersion:  runtime.Version(),
		Compiler:   runtime.Compiler,
		Platform:   fmt.Sprintf("%s/%s", runtime.GOOS, runtime.GOARCH),
	}
}

func convDate(timestamp string) string {
	ts, err := strconv.Atoi(timestamp)
	if err != nil {
		return undefined
	}
	return time.Unix(int64(ts), 0).UTC().Format(time.RFC3339)
}
