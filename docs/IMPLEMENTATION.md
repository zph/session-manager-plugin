# Session Manager Plugin Implementation Documentation

## Overview
This document tracks the implementation details and design decisions for the AWS SSM Session Manager Plugin.

## Components

### ssm-port-forward
A command-line tool providing SSH-style port forwarding syntax for AWS SSM sessions.

#### Signal Handling Implementation

**Specification:** See [docs/specs/signal-handling.md](specs/signal-handling.md)

**Implementation Status:** ✅ Complete

**Key Features:**
- Listens for SIGINT, SIGTERM, and SIGHUP signals
- Performs graceful shutdown with proper cleanup
- Implements 5-second timeout to prevent hanging
- Handles cleanup in both normal and error scenarios
- Active signal handling regardless of --wait flag setting

**Code References:**
- Main signal setup: `src/ssm-port-forward-main/main.go:208-210`
- Signal handling loop: `src/ssm-port-forward-main/main.go:329-339`
- Cleanup function: `src/ssm-port-forward-main/main.go:342-379`
- Unit tests: `src/ssm-port-forward-main/main_test.go`

**Implementation Details:**

1. **Signal Registration (SIGNAL-001, SIGNAL-002, SIGNAL-003)**
   - Buffered channel prevents signal loss during handler initialization
   - Registers for SIGINT, SIGTERM, and SIGHUP
   - Channel buffer size: 1

2. **Graceful Shutdown (SIGNAL-004, SIGNAL-005, SIGNAL-006)**
   - Closes websocket data channel via `DataChannel.Close()`
   - Ends SSM session via `DataChannel.EndSession()`
   - Returns status code 0 on successful cleanup

3. **Cleanup Timeout (SIGNAL-009)**
   - 5-second maximum for cleanup operations
   - Forces exit if timeout is exceeded
   - Prevents zombie processes

4. **Always-Active Signal Handling (SIGNAL-007, SIGNAL-008)**
   - Signal handling is active regardless of --wait flag
   - Removed early exit when --wait is false
   - Ensures proper cleanup in all execution modes

5. **Error Recovery (SIGNAL-010)**
   - Cleanup runs on both signal receipt and session errors
   - Logs cleanup errors without failing the main error path
   - Defensive programming for partial state scenarios

**Testing:**
- Unit tests verify signal buffering, timeout behavior, and multi-signal handling
- All tests pass with coverage of core signal handling paths
- Manual verification recommended for end-to-end signal behavior

**Trade-offs:**
- Changed behavior: Process now always waits for signal/error instead of exiting immediately when --wait=false
- Rationale: Prevents orphaned goroutines and ensures proper cleanup
- Impact: Users expecting immediate exit with --wait=false will see different behavior

**Future Enhancements:**
- Consider adding metrics for cleanup duration
- Add integration tests with actual SSM sessions
- Consider making cleanup timeout configurable

## Development Guidelines

### Testing Approach
This project follows Test-Driven Development (TDD):
1. Write EARS specification for new features
2. Write tests first (Red)
3. Implement feature (Green)
4. Refactor if needed (Refactor)

### Documentation
- All requirements must have EARS specifications in `docs/specs/`
- Code must include EARS spec tags (e.g., `// SIGNAL-001`)
- Update this IMPLEMENTATION.md when completing significant work

### Code Standards
- Use constants for magic values
- Prefer `iota` for enumerated constants
- Never ignore errors in Go code
- Mark incomplete implementations with TODO comments

#### Port Readiness Detection

**Specification:** See [docs/specs/port-ready.md](specs/port-ready.md)

**Implementation Status:** ✅ Complete

**Key Features:**
- `-w/--wait` now waits for both local TCP listener AND remote port readiness
- Uses `StartPublicationMessage` as positive readiness signal from SSM agent
- Detects `ConnectToPortError` flag messages and reports failure immediately
- Flag messages no longer leak raw binary data to output streams (bug fix)

**Code References:**
- `waitForReady()`: `src/ssm-port-forward-main/main.go` (replaces `waitForPort()`)
- Session channels: `src/sessionmanagerplugin/session/session.go` (`PortReady`, `PortError`)
- Flag handling: `src/sessionmanagerplugin/session/portsession/portsession.go` (`handleFlagMessage`)
- StartPublication channel: `src/datachannel/streaming.go` (`startPublicationReceived`)
- Unit tests: `src/ssm-port-forward-main/main_test.go`, `portsession_test.go`, `streaming_test.go`

**Implementation Details:**

1. **Positive Readiness Detection (READY-007, READY-008)**
   - DataChannel exposes `StartPublicationReceived` channel, closed when agent sends `start_publication`
   - PortSession watches this channel and closes `Session.PortReady` when fired
   - `waitForReady()` blocks on `PortReady` after local port is confirmed up

2. **ConnectToPortError Handling (READY-003, READY-005, READY-006)**
   - `ProcessStreamMessagePayload` now intercepts `PayloadType == Flag` messages
   - Decodes flag value from 4-byte big-endian payload
   - `ConnectToPortError` sends error to `Session.PortError` channel
   - Flag bytes are never written to the output stream (fixes data corruption bug)

3. **Two-Phase Wait (READY-001, READY-002, READY-008)**
   - Phase 1: Poll local TCP port until listener accepts connections
   - Phase 2: Wait for `PortReady` (success), `PortError` (failure), or timeout
   - Both phases watch `PortError` for early failure detection

**Testing:**
- `TestStartPublicationMessageClosesChannel` — datachannel signals on start_publication
- `TestStartPublicationMessageIdempotent` — double start_publication doesn't panic
- `TestProcessStreamMessagePayloadFlagNotWrittenToStream` — flag bytes not written to stream
- `TestProcessStreamMessagePayloadConnectToPortError` — error sent to PortError
- `TestInitializeSignalsReadyOnStartPublication` — PortReady closed on start_publication
- `TestWaitForReadyLocalPortAndRemoteReady` — happy path
- `TestWaitForReadyConnectToPortError` — ConnectToPortError fails wait
- `TestWaitForReadyTimeout` — timeout fails wait
- `TestWaitForReadyErrorBeforeLocalPort` — error before local port fails wait

**Trade-offs:**
- Relies on SSM agent sending `start_publication` for positive detection. If an older agent doesn't send it, `-w` will timeout instead of succeeding.
- `--timeout` (default 30s) bounds the wait for agents that never send readiness signals.

## Recent Changes

### 2026-03-12: Port Readiness Detection for `-w` flag
- **What:** `-w/--wait` now waits for end-to-end tunnel readiness, not just local port
- **Why:** Previous implementation only checked local TCP listener, missing remote connection failures
- **How:** Wired up `start_publication` as positive signal, intercept `ConnectToPortError` flags, two-phase wait
- **Testing:** 9 new unit tests across 3 packages, all pass with race detector
- **Bug Fix:** Flag messages no longer write raw binary to output streams
- **Specification:** docs/specs/port-ready.md
- **Tag Range:** READY-001 through READY-008

### 2026-03-12: Fix MuxClient/MgsConn nil pointer panic on close
- **What:** Added nil guards to `MuxClient.close()`, `MgsConn.close()`, and `MuxPortForwarding.Stop()`
- **Why:** `MuxClient.localListener` is initialized as nil in `initialize()` and only set later in `handleClientConnections()`. If `Stop()` is called before that (e.g., session ends early), the nil dereference panics goroutine 63.
- **How:** Added nil checks before calling `Close()` on each field in both `close()` methods
- **Testing:** Added MUX-001, MUX-002, MUX-003 unit tests verifying no panic with nil fields
- **Files:** `src/sessionmanagerplugin/session/portsession/muxportforwarding.go:67-82`, `muxportforwarding_test.go`
- **Tag Range:** MUX-001 through MUX-003

### 2025-12-13: Signal Handling Enhancement
- **What:** Added comprehensive signal handling to ssm-port-forward
- **Why:** Ensure rapid and graceful shutdown on interrupts
- **How:** Implemented cleanup function with timeout and proper session teardown
- **Testing:** Added unit tests, verified compilation and test pass
- **Specification:** docs/specs/signal-handling.md
- **Tag Range:** SIGNAL-001 through SIGNAL-010
