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

package restore

import (
	"fmt"
	"net/url"
	"os"
	"path"
	"strconv"
	"strings"

	"github.com/facebook/fbthrift/thrift/lib/go/thrift"
	pb "github.com/vesoft-inc/nebula-agent/pkg/proto"
	"github.com/vesoft-inc/nebula-go/v3/nebula"
	"github.com/vesoft-inc/nebula-go/v3/nebula/meta"
)

const LocalTmpDir = "/tmp/nebula-br"

type Config struct {
	BackupName  string
	Concurrency int32
	Backend     *pb.Backend
}

func ParseAddr(addrStr string) (*nebula.HostAddr, error) {
	ipAddr := strings.Split(addrStr, ":")
	if len(ipAddr) != 2 {
		return nil, fmt.Errorf("invalid address format: %s", addrStr)
	}

	port, err := strconv.ParseInt(ipAddr[1], 10, 32)
	if err != nil {
		return nil, err
	}

	return &nebula.HostAddr{Host: ipAddr[0], Port: nebula.Port(port)}, nil
}

func ParseMetaFromFile(filename string) (*meta.BackupMeta, error) {
	file, err := os.OpenFile(filename, os.O_RDONLY, 0644)
	if err != nil {
		return nil, err
	}
	defer file.Close()

	st := thrift.NewStreamTransport(file, file)
	defer st.Close()

	ft := thrift.NewFramedTransport(st)
	defer ft.Close()

	bp := thrift.NewBinaryProtocol(ft, false, true)
	m := meta.NewBackupMeta()
	if err = m.Read(bp); err != nil {
		return nil, err
	}
	return m, nil
}

func UriJoin(elem ...string) (string, error) {
	if len(elem) == 0 {
		return "", fmt.Errorf("empty paths")
	}

	if len(elem) == 1 {
		return elem[0], nil
	}

	u, err := url.Parse(elem[0])
	if err != nil {
		return "", fmt.Errorf("parse base uri %s failed: %v", elem[0], err)
	}

	elem[0] = u.Path
	u.Path = path.Join(elem...)
	return u.String(), nil
}

func StringifyAddr(addr *nebula.HostAddr) string {
	if addr == nil {
		return "nil"
	}
	return fmt.Sprintf("%s:%d", addr.GetHost(), addr.GetPort())
}
