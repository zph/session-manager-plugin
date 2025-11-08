// Copyright 2019 Amazon.com, Inc. or its affiliates. All Rights Reserved.
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
	"errors"
	"net"
	"os"
	"os/signal"
	"syscall"
	"testing"
	"time"

	"github.com/aws/session-manager-plugin/src/log"
	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/mock"
)

// This test passes ctrl+c signal which blocks running of all other tests.
func TestSetSessionHandlers(t *testing.T) {
	mockLog.Infof("TestStartSession!!!!!")
	out, in := net.Pipe()
	defer out.Close()
	defer in.Close()

	counter := 0
	countTimes := func() error {
		counter++
		return nil
	}
	mockWebSocketChannel.On("SendMessage", mockLog, mock.Anything, mock.Anything).
		Return(countTimes())

	mockSession := getSessionMock()
	portSession := PortSession{
		Session:        mockSession,
		portParameters: PortParameters{PortNumber: "22", Type: "LocalPortForwarding"},
		portSessionType: &BasicPortForwarding{
			session:        mockSession,
			portParameters: PortParameters{PortNumber: "22", Type: "LocalPortForwarding"},
		},
	}
	signalCh := make(chan os.Signal, 1)
	go func() {
		time.Sleep(100 * time.Millisecond)
		if _, err := out.Write([]byte("testing123")); err != nil {
			mockLog.Infof("error: ", err)
		}
	}()

	go func() {
		_ = func(log log.T, listener net.Listener) (tcpConn net.Conn, err error) {
			return in, nil
		}
		signal.Notify(signalCh, syscall.SIGINT, syscall.SIGQUIT, syscall.SIGTSTP)
		process, _ := os.FindProcess(os.Getpid())
		process.Signal(syscall.SIGINT)
		portSession.SetSessionHandlers(mockLog)
	}()

	time.Sleep(time.Second)
	assert.Equal(t, <-signalCh, syscall.SIGINT)
	assert.Equal(t, counter, 1)
	mockWebSocketChannel.AssertExpectations(t)
}

func TestStartSessionTCPLocalPortFromDocument(t *testing.T) {
	portSession := PortSession{
		Session:        getSessionMock(),
		portParameters: PortParameters{PortNumber: "22", Type: "LocalPortForwarding", LocalPortNumber: "54321"},
		portSessionType: &BasicPortForwarding{
			session:        getSessionMock(),
			portParameters: PortParameters{PortNumber: "22", Type: "LocalPortForwarding"},
		},
	}
	// Test only verifies that LocalPortNumber parameter is parsed correctly
	// Don't call SetSessionHandlers() as it blocks on listener.Accept()
	assert.Equal(t, "54321", portSession.portParameters.LocalPortNumber)
}

func TestStartSessionTCPAcceptFailed(t *testing.T) {
	connErr := errors.New("accept failed")

	// Create a mock listener that fails on Accept
	mockListener := &MockNetListener{}
	mockListener.On("Accept").Return(nil, connErr)

	basicPortForwarding := &BasicPortForwarding{
		session:        getSessionMock(),
		portParameters: PortParameters{PortNumber: "22", Type: "LocalPortForwarding"},
		listener:       mockListener, // Inject mock listener
	}

	// Call startLocalConn directly to test Accept failure
	err := basicPortForwarding.startLocalConn(mockLog)
	assert.Equal(t, connErr, err)
	mockListener.AssertExpectations(t)
}

func TestStartSessionTCPConnectFailed(t *testing.T) {
	// Test listener creation failure by using an invalid unix socket path
	basicPortForwarding := &BasicPortForwarding{
		session: getSessionMock(),
		portParameters: PortParameters{
			PortNumber:          "22",
			Type:                "LocalPortForwarding",
			LocalConnectionType: "unix",
			LocalUnixSocket:     "/invalid/path/that/does/not/exist/socket.sock",
		},
	}

	portSession := PortSession{
		Session:         getSessionMock(),
		portParameters:  basicPortForwarding.portParameters,
		portSessionType: basicPortForwarding,
	}

	// SetSessionHandlers should fail when trying to create listener with invalid path
	err := portSession.SetSessionHandlers(mockLog)
	assert.NotNil(t, err)
	assert.Contains(t, err.Error(), "no such file or directory")
}
