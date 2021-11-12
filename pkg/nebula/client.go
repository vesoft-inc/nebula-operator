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

package nebula

import (
	"math"

	"github.com/facebook/fbthrift/thrift/lib/go/thrift"
)

const (
	defaultBufferSize = 128 << 10
	frameMaxLength    = math.MaxUint32
)

func buildClientTransport(endpoint string, options ...Option) (thrift.Transport, thrift.ProtocolFactory, error) {
	opts := loadOptions(options...)
	timeoutOption := thrift.SocketTimeout(opts.Timeout)
	addressOption := thrift.SocketAddr(endpoint)
	sock, err := thrift.NewSocket(timeoutOption, addressOption)
	if err != nil {
		return nil, nil, err
	}
	bufferedTranFactory := thrift.NewBufferedTransportFactory(defaultBufferSize)
	transport := thrift.NewFramedTransportMaxLength(bufferedTranFactory.GetTransport(sock), frameMaxLength)
	pf := thrift.NewBinaryProtocolFactoryDefault()

	return transport, pf, nil
}
