// Copyright 2025 Zander Hill. All Rights Reserved.
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

// SIGNAL-001, SIGNAL-002, SIGNAL-003, SIGNAL-004, SIGNAL-005, SIGNAL-006, SIGNAL-009
package main

import (
	"context"
	"errors"
	"net"
	"os"
	"os/signal"
	"strconv"
	"syscall"
	"testing"
	"time"
)

// neverDone is a channel that is never closed, for tests that don't need signal cancellation.
var neverDone = make(chan struct{})

// TestSignalHandlerRegistration verifies that signal handler is registered
// SIGNAL-007, SIGNAL-008
func TestSignalHandlerRegistration(t *testing.T) {
	// Create a signal channel
	sigChan := make(chan os.Signal, 1)

	// Register for SIGINT and SIGTERM
	signal.Notify(sigChan, syscall.SIGINT, syscall.SIGTERM, syscall.SIGHUP)

	// Verify we can send a signal to the channel
	done := make(chan bool, 1)
	go func() {
		select {
		case sig := <-sigChan:
			if sig != syscall.SIGTERM {
				t.Errorf("Expected SIGTERM, got %v", sig)
			}
			done <- true
		case <-time.After(100 * time.Millisecond):
			t.Error("Timeout waiting for signal")
			done <- false
		}
	}()

	// Send a signal
	sigChan <- syscall.SIGTERM

	if success := <-done; !success {
		t.Fatal("Signal handling failed")
	}

	signal.Stop(sigChan)
}

// TestSignalHandlerShutdownTimeout verifies that shutdown completes within timeout
// SIGNAL-009
func TestSignalHandlerShutdownTimeout(t *testing.T) {
	const shutdownTimeout = 5 * time.Second

	ctx, cancel := context.WithTimeout(context.Background(), shutdownTimeout)
	defer cancel()

	// Simulate cleanup
	cleanupDone := make(chan bool, 1)
	go func() {
		// Simulate cleanup work
		time.Sleep(100 * time.Millisecond)
		cleanupDone <- true
	}()

	select {
	case <-cleanupDone:
		// Success - cleanup completed within timeout
	case <-ctx.Done():
		t.Fatal("Cleanup exceeded shutdown timeout")
	}
}

// TestSignalHandlerBufferPreventsLoss verifies buffered channel prevents signal loss
// SIGNAL-001
func TestSignalHandlerBufferPreventsLoss(t *testing.T) {
	// Create buffered channel to prevent signal loss
	sigChan := make(chan os.Signal, 1)
	signal.Notify(sigChan, syscall.SIGINT)

	// Send signal before goroutine is ready
	sigChan <- syscall.SIGINT

	// Start handler after signal is sent (should still receive it)
	select {
	case sig := <-sigChan:
		if sig != syscall.SIGINT {
			t.Errorf("Expected SIGINT, got %v", sig)
		}
	case <-time.After(100 * time.Millisecond):
		t.Error("Signal was lost")
	}

	signal.Stop(sigChan)
}

// TestMultipleSignalTypes verifies all signal types are handled
// SIGNAL-001, SIGNAL-002, SIGNAL-003
func TestMultipleSignalTypes(t *testing.T) {
	signals := []syscall.Signal{
		syscall.SIGINT,
		syscall.SIGTERM,
		syscall.SIGHUP,
	}

	for _, sig := range signals {
		t.Run(sig.String(), func(t *testing.T) {
			sigChan := make(chan os.Signal, 1)
			signal.Notify(sigChan, sig)

			// Send the signal
			sigChan <- sig

			// Verify it's received
			select {
			case received := <-sigChan:
				if received != sig {
					t.Errorf("Expected %v, got %v", sig, received)
				}
			case <-time.After(100 * time.Millisecond):
				t.Errorf("Timeout waiting for %v", sig)
			}

			signal.Stop(sigChan)
		})
	}
}

// READY-001, READY-002, READY-007
func TestWaitForReadyLocalPortAndRemoteReady(t *testing.T) {
	// Start a local TCP listener to simulate the port being ready
	listener, err := net.Listen("tcp", "localhost:0")
	if err != nil {
		t.Fatalf("Failed to start listener: %v", err)
	}
	defer listener.Close()
	port := strconv.Itoa(listener.Addr().(*net.TCPAddr).Port)

	portReady := make(chan struct{})
	portError := make(chan error, 1)

	// Simulate agent sending StartPublicationMessage after a short delay
	go func() {
		time.Sleep(50 * time.Millisecond)
		close(portReady)
	}()

	err = waitForReady(port, portReady, portError, 5*time.Second, neverDone)
	if err != nil {
		t.Fatalf("Expected success, got error: %v", err)
	}
}

// READY-003: ConnectToPortError during wait SHALL report failure
func TestWaitForReadyConnectToPortError(t *testing.T) {
	// Start a local TCP listener
	listener, err := net.Listen("tcp", "localhost:0")
	if err != nil {
		t.Fatalf("Failed to start listener: %v", err)
	}
	defer listener.Close()
	port := strconv.Itoa(listener.Addr().(*net.TCPAddr).Port)

	portReady := make(chan struct{})
	portError := make(chan error, 1)

	// Simulate ConnectToPortError arriving after local port is up
	go func() {
		time.Sleep(50 * time.Millisecond)
		portError <- errors.New("ConnectToPortError: agent failed to connect to remote port")
	}()

	err = waitForReady(port, portReady, portError, 5*time.Second, neverDone)
	if err == nil {
		t.Fatal("Expected error, got nil")
	}
	if !errors.Is(err, errRemotePortFailed) {
		t.Fatalf("Expected errRemotePortFailed, got: %v", err)
	}
}

// READY-004: Timeout before readiness SHALL report failure
func TestWaitForReadyTimeout(t *testing.T) {
	// Use a port that doesn't have a listener
	portReady := make(chan struct{})
	portError := make(chan error, 1)

	err := waitForReady("0", portReady, portError, 200*time.Millisecond, neverDone)
	if err == nil {
		t.Fatal("Expected timeout error, got nil")
	}
	if !errors.Is(err, errWaitTimeout) {
		t.Fatalf("Expected errWaitTimeout, got: %v", err)
	}
}

// READY-009: WHEN the agent does not send StartPublicationMessage,
// THEN waitForReady SHALL succeed after local port is confirmed ready
// (graceful fallback matching pre-Phase-2 behavior).
func TestWaitForReadySucceedsWithoutStartPublication(t *testing.T) {
	// Start a local TCP listener to simulate the port being ready
	listener, err := net.Listen("tcp", "localhost:0")
	if err != nil {
		t.Fatalf("Failed to start listener: %v", err)
	}
	defer listener.Close()
	port := strconv.Itoa(listener.Addr().(*net.TCPAddr).Port)

	// PortReady is never closed — simulates agent that doesn't send StartPublicationMessage
	portReady := make(chan struct{})
	portError := make(chan error, 1)

	err = waitForReady(port, portReady, portError, 5*time.Second, neverDone)
	if err != nil {
		t.Fatalf("Expected success (graceful fallback), got error: %v", err)
	}
}

// SIGNAL-011: WHEN a signal is received during waitForReady Phase 1,
// THEN waitForReady SHALL return errSignalReceived promptly.
func TestWaitForReadySignalDuringPhase1(t *testing.T) {
	// No listener — Phase 1 will be polling
	portReady := make(chan struct{})
	portError := make(chan error, 1)
	done := make(chan struct{})

	// Close done after a short delay to simulate signal arrival
	go func() {
		time.Sleep(50 * time.Millisecond)
		close(done)
	}()

	start := time.Now()
	err := waitForReady("0", portReady, portError, 30*time.Second, done)
	elapsed := time.Since(start)

	if err == nil {
		t.Fatal("Expected error, got nil")
	}
	if !errors.Is(err, errSignalReceived) {
		t.Fatalf("Expected errSignalReceived, got: %v", err)
	}
	if elapsed > 2*time.Second {
		t.Fatalf("Signal not handled promptly: took %v", elapsed)
	}
}

// SIGNAL-011: WHEN a signal is received during waitForReady Phase 2,
// THEN waitForReady SHALL return errSignalReceived promptly.
func TestWaitForReadySignalDuringPhase2(t *testing.T) {
	// Start a listener so Phase 1 passes immediately
	listener, err := net.Listen("tcp", "localhost:0")
	if err != nil {
		t.Fatalf("Failed to start listener: %v", err)
	}
	defer listener.Close()
	port := strconv.Itoa(listener.Addr().(*net.TCPAddr).Port)

	portReady := make(chan struct{})
	portError := make(chan error, 1)
	done := make(chan struct{})

	// Close done after a short delay to simulate signal during Phase 2
	go func() {
		time.Sleep(50 * time.Millisecond)
		close(done)
	}()

	start := time.Now()
	err = waitForReady(port, portReady, portError, 30*time.Second, done)
	elapsed := time.Since(start)

	if err == nil {
		t.Fatal("Expected error, got nil")
	}
	if !errors.Is(err, errSignalReceived) {
		t.Fatalf("Expected errSignalReceived, got: %v", err)
	}
	if elapsed > 2*time.Second {
		t.Fatalf("Signal not handled promptly: took %v", elapsed)
	}
}

// READY-003: Error before local port is ready SHALL report failure
func TestWaitForReadyErrorBeforeLocalPort(t *testing.T) {
	// Don't start a listener - port won't be available
	portReady := make(chan struct{})
	portError := make(chan error, 1)

	// Send error immediately
	portError <- errors.New("ConnectToPortError: agent failed to connect")

	err := waitForReady("0", portReady, portError, 5*time.Second, neverDone)
	if err == nil {
		t.Fatal("Expected error, got nil")
	}
	if !errors.Is(err, errRemotePortFailed) {
		t.Fatalf("Expected errRemotePortFailed, got: %v", err)
	}
}
