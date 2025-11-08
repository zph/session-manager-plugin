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

// shellsession starts shell session.
package shellsession

import (
	"sync"
	"testing"
	"time"

	"github.com/aws/session-manager-plugin/src/datachannel"
	"github.com/aws/session-manager-plugin/src/log"
	"github.com/aws/session-manager-plugin/src/message"
	"github.com/aws/session-manager-plugin/src/sessionmanagerplugin/session"
	"github.com/stretchr/testify/assert"
)

func TestTerminalResizeWhenSessionSizeDataIsNotEqualToActualSize(t *testing.T) {
	dataChannel := getDataChannel()

	session := session.Session{
		DataChannel: dataChannel,
	}

	sizeData := message.SizeData{
		Cols: 100,
		Rows: 100,
	}

	shellSession := ShellSession{
		Session:  session,
		SizeData: sizeData,
	}
	GetTerminalSizeCall = func(fd int) (width int, height int, err error) {
		return 123, 123, nil
	}

	var wg sync.WaitGroup
	wg.Add(1)
	// Spawning a separate go routine to close websocket connection.
	// This is required as handleTerminalResize has a for loop which will continuously check for
	// size data every 500ms.
	go func() {
		time.Sleep(1 * time.Second)
		wg.Done()
	}()

	SendMessageCallCount := 0
	datachannel.SendMessageCall = func(log log.T, dataChannel *datachannel.DataChannel, input []byte, inputType int) error {
		SendMessageCallCount++
		return nil
	}
	shellSession.handleTerminalResize(logger)
	wg.Wait()
	assert.True(t, SendMessageCallCount > 0)
}
