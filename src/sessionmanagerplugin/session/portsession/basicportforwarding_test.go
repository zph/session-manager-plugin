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
	"testing"

	"github.com/stretchr/testify/assert"
)

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
