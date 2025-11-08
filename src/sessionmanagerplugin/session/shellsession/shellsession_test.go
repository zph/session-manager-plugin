// Copyright 2018 Amazon.com, Inc. or its affiliates. All Rights Reserved.
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

// Package shellsession starts shell session.
package shellsession

import (
	"encoding/json"
	"fmt"
	"os"
	"os/signal"
	"syscall"
	"testing"
	"time"

	"github.com/zph/session-manager-plugin/src/communicator/mocks"
	"github.com/zph/session-manager-plugin/src/datachannel"
	dataChannelMock "github.com/zph/session-manager-plugin/src/datachannel/mocks"
	"github.com/zph/session-manager-plugin/src/log"
	"github.com/zph/session-manager-plugin/src/message"
	"github.com/zph/session-manager-plugin/src/sessionmanagerplugin/session"
	"github.com/zph/session-manager-plugin/src/sessionmanagerplugin/session/sessionutil"
	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/mock"
)

var (
	expectedSequenceNumber = int64(0)
	logger                 = log.NewMockLog()
	clientId               = "clientId"
	sessionId              = "sessionId"
	instanceId             = "instanceId"
	mockDataChannel        = &dataChannelMock.IDataChannel{}
	mockWsChannel          = &mocks.IWebSocketChannel{}
)

func TestName(t *testing.T) {
	shellSession := ShellSession{}
	name := shellSession.Name()
	assert.Equal(t, name, "Standard_Stream")
}

func TestInitialize(t *testing.T) {
	session := &session.Session{}
	shellSession := ShellSession{}
	session.DataChannel = mockDataChannel
	mockDataChannel.On("RegisterOutputStreamHandler", mock.Anything, true).Times(1)
	mockDataChannel.On("GetWsChannel").Return(mockWsChannel)
	mockWsChannel.On("SetOnMessage", mock.Anything)
	shellSession.Initialize(logger, session)
	assert.Equal(t, shellSession.Session, *session)
}

func TestHandleControlSignals(t *testing.T) {
	session := session.Session{}
	session.DataChannel = mockDataChannel
	shellSession := ShellSession{}
	shellSession.Session = session

	waitCh := make(chan int, 1)
	counter := 0
	sendDataMessage := func() error {
		counter++
		return fmt.Errorf("SendInputDataMessage error")
	}
	mockDataChannel.On("SendInputDataMessage", mock.Anything, mock.Anything, mock.Anything).Return(sendDataMessage())

	signalCh := make(chan os.Signal, 1)
	go func() {
		p, _ := os.FindProcess(os.Getpid())
		signal.Notify(signalCh, syscall.SIGINT, syscall.SIGQUIT, syscall.SIGTSTP)
		shellSession.handleControlSignals(logger)
		p.Signal(syscall.SIGINT)
		time.Sleep(200 * time.Millisecond)
		close(waitCh)
	}()

	<-waitCh
	assert.Equal(t, <-signalCh, syscall.SIGINT)
	assert.Equal(t, counter, 1)
}

func TestSendInputDataMessageWithPayloadTypeSize(t *testing.T) {
	sizeData := message.SizeData{
		Cols: 100,
		Rows: 100,
	}
	sizeDataBytes, _ := json.Marshal(sizeData)
	dataChannel := getDataChannel()
	mockChannel := &mocks.IWebSocketChannel{}
	dataChannel.SetWsChannel(mockChannel)
	SendMessageCallCount := 0
	datachannel.SendMessageCall = func(log log.T, dataChannel *datachannel.DataChannel, input []byte, inputType int) error {
		SendMessageCallCount++
		return nil
	}

	err := dataChannel.SendInputDataMessage(logger, message.Size, sizeDataBytes)
	assert.Nil(t, err)
	assert.Equal(t, expectedSequenceNumber, dataChannel.ExpectedSequenceNumber)
	assert.Equal(t, 1, SendMessageCallCount)
}

func TestProcessStreamMessagePayload(t *testing.T) {
	shellSession := ShellSession{}
	shellSession.DisplayMode = sessionutil.NewDisplayMode(logger)

	msg := message.ClientMessage{
		Payload: []byte("Hello Agent\n"),
	}
	isReady, err := shellSession.ProcessStreamMessagePayload(logger, msg)
	assert.True(t, isReady)
	assert.Nil(t, err)
}

func getDataChannel() *datachannel.DataChannel {
	dataChannel := &datachannel.DataChannel{}
	dataChannel.Initialize(logger, clientId, sessionId, instanceId, false)
	dataChannel.SetWsChannel(mockWsChannel)
	return dataChannel
}
