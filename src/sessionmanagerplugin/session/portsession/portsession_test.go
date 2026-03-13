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
	"encoding/binary"
	"io/ioutil"
	"os"
	"testing"
	"time"

	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/mock"
	"github.com/zph/session-manager-plugin/src/communicator/mocks"
	"github.com/zph/session-manager-plugin/src/datachannel"
	"github.com/zph/session-manager-plugin/src/jsonutil"
	"github.com/zph/session-manager-plugin/src/message"
)

// Test Initialize
func TestInitializePortSession(t *testing.T) {
	// Reset mock expectations before test
	mockWebSocketChannel = mocks.IWebSocketChannel{}
	t.Cleanup(func() {
		mockWebSocketChannel = mocks.IWebSocketChannel{}
	})

	var portParameters PortParameters
	jsonutil.Remarshal(properties, &portParameters)

	mockWebSocketChannel.On("SetOnMessage", mock.Anything)

	portSession := PortSession{
		Session: getSessionMock(),
	}
	portSession.Initialize(mockLog, &portSession.Session)

	mockWebSocketChannel.AssertExpectations(t)
	assert.Equal(t, portParameters, portSession.portParameters, "Initialize port parameters")
	assert.IsType(t, &StandardStreamForwarding{}, portSession.portSessionType)
}

func TestInitializePortSessionForPortForwardingWithOldAgent(t *testing.T) {
	// Reset mock expectations before test
	mockWebSocketChannel = mocks.IWebSocketChannel{}
	t.Cleanup(func() {
		mockWebSocketChannel = mocks.IWebSocketChannel{}
	})

	var portParameters PortParameters
	jsonutil.Remarshal(map[string]interface{}{"portNumber": "8080", "type": "LocalPortForwarding"}, &portParameters)

	mockWebSocketChannel.On("SetOnMessage", mock.Anything)

	portSession := PortSession{
		Session: getSessionMockWithParams(portParameters, "2.2.0.0"),
	}
	portSession.Initialize(mockLog, &portSession.Session)

	mockWebSocketChannel.AssertExpectations(t)
	assert.Equal(t, portParameters, portSession.portParameters, "Initialize port parameters")
	assert.IsType(t, &BasicPortForwarding{}, portSession.portSessionType)
}

func TestInitializePortSessionForPortForwarding(t *testing.T) {
	// Reset mock expectations before test
	mockWebSocketChannel = mocks.IWebSocketChannel{}
	t.Cleanup(func() {
		mockWebSocketChannel = mocks.IWebSocketChannel{}
	})

	var portParameters PortParameters
	jsonutil.Remarshal(map[string]interface{}{"portNumber": "8080", "type": "LocalPortForwarding"}, &portParameters)

	mockWebSocketChannel.On("SetOnMessage", mock.Anything)

	portSession := PortSession{
		Session: getSessionMockWithParams(portParameters, "3.1.0.0"),
	}
	portSession.Initialize(mockLog, &portSession.Session)

	mockWebSocketChannel.AssertExpectations(t)
	assert.Equal(t, portParameters, portSession.portParameters, "Initialize port parameters")
	assert.IsType(t, &MuxPortForwarding{}, portSession.portSessionType)
}

// Test ProcessStreamMessagePayload
func TestProcessStreamMessagePayload(t *testing.T) {
	in, out, _ := os.Pipe()
	defer func() {
		in.Close()
		out.Close()
	}()

	go func() {
		portSession := PortSession{
			Session:        getSessionMock(),
			portParameters: PortParameters{PortNumber: "22"},
			portSessionType: &StandardStreamForwarding{
				inputStream:  in,
				outputStream: out,
			},
		}
		portSession.ProcessStreamMessagePayload(mockLog, outputMessage)
		out.Close()
	}()

	payload, _ := ioutil.ReadAll(in)
	assert.Equal(t, outputMessage.Payload, payload)
}

// READY-005: Flag messages SHALL NOT be written to the output stream
func TestProcessStreamMessagePayloadFlagNotWrittenToStream(t *testing.T) {
	in, out, _ := os.Pipe()
	defer func() {
		in.Close()
		out.Close()
	}()

	sess := getSessionMock()
	sess.PortError = make(chan error, 1)

	// Build a flag message with DisconnectToPort (a non-error flag)
	flagPayload := make([]byte, 4)
	binary.BigEndian.PutUint32(flagPayload, uint32(message.DisconnectToPort))
	flagMessage := message.ClientMessage{
		PayloadType:   uint32(message.Flag),
		Payload:       flagPayload,
		PayloadLength: 4,
	}

	portSession := PortSession{
		Session:        sess,
		portParameters: PortParameters{PortNumber: "22"},
		portSessionType: &StandardStreamForwarding{
			inputStream:  in,
			outputStream: out,
		},
	}

	isReady, err := portSession.ProcessStreamMessagePayload(mockLog, flagMessage)
	assert.True(t, isReady)
	assert.Nil(t, err)

	// Verify nothing was written to the output stream
	out.Close()
	payload, _ := ioutil.ReadAll(in)
	assert.Empty(t, payload, "Flag message should not write bytes to output stream")
}

// READY-003, READY-006: ConnectToPortError SHALL send error to PortError channel
func TestProcessStreamMessagePayloadConnectToPortError(t *testing.T) {
	in, out, _ := os.Pipe()
	defer func() {
		in.Close()
		out.Close()
	}()

	sess := getSessionMock()
	sess.PortError = make(chan error, 1)

	flagPayload := make([]byte, 4)
	binary.BigEndian.PutUint32(flagPayload, uint32(message.ConnectToPortError))
	flagMessage := message.ClientMessage{
		PayloadType:   uint32(message.Flag),
		Payload:       flagPayload,
		PayloadLength: 4,
	}

	portSession := PortSession{
		Session:        sess,
		portParameters: PortParameters{PortNumber: "22"},
		portSessionType: &StandardStreamForwarding{
			inputStream:  in,
			outputStream: out,
		},
	}

	isReady, err := portSession.ProcessStreamMessagePayload(mockLog, flagMessage)
	assert.True(t, isReady)
	assert.Nil(t, err)

	// Verify error was sent to PortError
	select {
	case portErr := <-sess.PortError:
		assert.Contains(t, portErr.Error(), "ConnectToPortError")
	default:
		t.Fatal("Expected error on PortError channel for ConnectToPortError flag")
	}

	// Verify nothing was written to the output stream
	out.Close()
	payload, _ := ioutil.ReadAll(in)
	assert.Empty(t, payload, "ConnectToPortError should not write bytes to output stream")
}

// READY-007: Initialize SHALL watch for StartPublicationMessage and close PortReady
func TestInitializeSignalsReadyOnStartPublication(t *testing.T) {
	mockWebSocketChannel = mocks.IWebSocketChannel{}
	t.Cleanup(func() {
		mockWebSocketChannel = mocks.IWebSocketChannel{}
	})

	sess := getSessionMock()
	sess.PortReady = make(chan struct{})

	mockWebSocketChannel.On("SetOnMessage", mock.Anything)

	portSession := PortSession{
		Session: sess,
	}
	portSession.Initialize(mockLog, &portSession.Session)

	// Verify PortReady is not closed yet
	select {
	case <-portSession.Session.PortReady:
		t.Fatal("PortReady should not be closed before StartPublicationMessage")
	default:
		// expected
	}

	// Simulate StartPublicationMessage by closing the datachannel's channel
	// (In production, this happens when OutputMessageHandler processes start_publication)
	dc := portSession.Session.DataChannel.(*datachannel.DataChannel)
	close(dc.StartPublicationReceivedForTest())

	// Wait briefly for the goroutine to observe the close
	timer := time.NewTimer(100 * time.Millisecond)
	defer timer.Stop()
	select {
	case <-portSession.Session.PortReady:
		// expected - PortReady closed after StartPublicationMessage
	case <-timer.C:
		t.Fatal("PortReady should be closed after StartPublicationMessage is received")
	}
}
