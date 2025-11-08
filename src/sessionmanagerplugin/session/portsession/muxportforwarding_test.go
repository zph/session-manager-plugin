// Copyright 2020 Amazon.com, Inc. or its affiliates. All Rights Reserved.
//
// Licensed under the Apache License, Version 2.0 (the "License"). You may not
// use this file except in compliance with the License. A copy of the
// License is located at
//
// http://aws.amazon.com/apache2.0/
//
// or in the "license" file accompanying this file. This file is distributed
// on an "AS IS" BASIS, WITHOUT WARRANTIES OR CONDITIONS OF ANY KIND,
// either express or implied. See the License for the specific language governing
// permissions and limitations under the License.

// Package portsession starts port session.
package portsession

import (
	"net"
	"testing"

	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/mock"
)

// MockNetListener is a mock net.Listener for testing
type MockNetListener struct {
	mock.Mock
}

func (m *MockNetListener) Accept() (net.Conn, error) {
	args := m.Called()
	if args.Get(0) == nil {
		return nil, args.Error(1)
	}
	return args.Get(0).(net.Conn), args.Error(1)
}

func (m *MockNetListener) Close() error {
	args := m.Called()
	return args.Error(0)
}

func (m *MockNetListener) Addr() net.Addr {
	args := m.Called()
	return args.Get(0).(net.Addr)
}

// test writeStream
func TestWriteStream(t *testing.T) {
	out, in := net.Pipe()
	defer in.Close()
	defer out.Close()

	portSession := PortSession{
		portSessionType: &MuxPortForwarding{
			session: getSessionMock(),
			mgsConn: &MgsConn{nil, in},
		},
	}

	go func() {
		portSession.portSessionType.WriteStream(outputMessage)
	}()

	msg := make([]byte, 20)
	n, _ := out.Read(msg)
	msg = msg[:n]

	assert.Equal(t, outputMessage.Payload, msg)
}

// Test handleDataTransfer
func TestHandleDataTransferSrcToDst(t *testing.T) {
	msg := make([]byte, 20)
	out, in := net.Pipe()
	out1, in1 := net.Pipe()

	defer out1.Close()
	defer in.Close()
	defer out.Close()
	defer in1.Close()

	done := make(chan bool)
	go func() {
		in.Write(outputMessage.Payload)
		in.Close()
	}()
	go func() {
		n, _ := out1.Read(msg)
		msg = msg[:n]
		done <- true
	}()

	handleDataTransfer(in1, out)
	<-done // Wait for read goroutine to complete
	assert.EqualValues(t, outputMessage.Payload, msg)
}

func TestHandleDataTransferDstToSrc(t *testing.T) {
	msg := make([]byte, 20)
	out, in := net.Pipe()
	out1, in1 := net.Pipe()

	defer out.Close()
	defer in.Close()
	defer out1.Close()
	defer in1.Close()

	done := make(chan bool)
	go func() {
		in1.Write(outputMessage.Payload)
		in1.Close()
	}()
	go func() {
		n, _ := out.Read(msg)
		msg = msg[:n]
		done <- true
	}()

	handleDataTransfer(in, out1)
	<-done // Wait for read goroutine to complete
	assert.EqualValues(t, outputMessage.Payload, msg)
}
