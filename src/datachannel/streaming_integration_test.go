//go:build integration

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

// datachannel package implement data channel which is used to interactively communicate with remote instance.
package datachannel

import (
	"sync"
	"testing"
	"time"

	communicatorMocks "github.com/aws/session-manager-plugin/src/communicator/mocks"
	"github.com/aws/session-manager-plugin/src/log"
	"github.com/stretchr/testify/assert"
)

func TestResendStreamDataMessageScheduler(t *testing.T) {
	dataChannel := getDataChannel()
	mockChannel := &communicatorMocks.IWebSocketChannel{}
	dataChannel.wsChannel = mockChannel
	for i := 0; i < 3; i++ {
		dataChannel.AddDataToOutgoingMessageBuffer(streamingMessages[i])
	}

	var wg sync.WaitGroup
	wg.Add(1)
	// Spawning a separate go routine to close websocket connection.
	// This is required as ResendStreamDataMessageScheduler has a for loop which will continuosly resend data until channel is closed.
	go func() {
		time.Sleep(1 * time.Second)
		wg.Done()
	}()

	SendMessageCallCount := 0
	SendMessageCall = func(log log.T, dataChannel *DataChannel, input []byte, inputType int) error {
		SendMessageCallCount++
		return nil
	}
	dataChannel.ResendStreamDataMessageScheduler(mockLogger)
	wg.Wait()
	assert.True(t, SendMessageCallCount > 1)
}
