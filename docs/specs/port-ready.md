# Port Readiness Detection Requirements

## Overview

This document specifies the requirements for ensuring the `-w/--wait` flag on `ssm-port-forward` truly waits until the remote port is ready before reporting success. The current implementation only verifies the local TCP listener is accepting connections, but does not confirm the SSM agent has connected to the remote port. This specification defines positive readiness detection using the SSM protocol's `start_publication` message and negative detection via `ConnectToPortError` flag messages.

**System Name:** SSM Port Forward CLI
**Tag Prefix:** READY
**Version:** 1.0
**Last Updated:** 2026-03-12

## Requirements

### Handshake Readiness

**READY-001:** Complex Event Driven

**Requirement:**
WHEN the `-w` flag is set AND the SSM session is started, the SSM Port Forward CLI SHALL wait until the SSM handshake is complete before reporting readiness.

**Rationale:**
The handshake establishes the session type and properties. Without a completed handshake, the port forwarding session type has not been determined and streams cannot be initialized.

**Verification:**
Test by starting a session with `-w` and verifying that readiness is not reported before the `HandshakeComplete` message is received.

---

### Local Listener Readiness

**READY-002:** Complex Event Driven

**Requirement:**
WHEN the `-w` flag is set AND the SSM handshake is complete, the SSM Port Forward CLI SHALL wait until the local TCP listener accepts connections before reporting readiness.

**Rationale:**
Callers relying on `-w` need to connect to the local port immediately after readiness is reported. The local listener MUST be accepting connections at that point.

**Verification:**
Test by starting a session with `-w` and verifying that `net.Dial` to the local port succeeds only after readiness is reported.

---

### Remote Port Error Detection

**READY-003:** Complex Unwanted Behaviour

**Requirement:**
While the `-w` flag is set AND the system is waiting for readiness, IF the agent sends a `ConnectToPortError` flag message, THEN the SSM Port Forward CLI SHALL report failure with a descriptive error.

**Rationale:**
A `ConnectToPortError` means the SSM agent cannot reach the remote port. Reporting readiness in this state would cause the caller's first connection to fail silently or with confusing errors.

**Verification:**
Test by injecting a `ConnectToPortError` flag message during the wait period and verifying the CLI exits with a non-zero status and descriptive error message.

---

### Timeout Handling

**READY-004:** Complex Unwanted Behaviour

**Requirement:**
While the `-w` flag is set AND the system is waiting for readiness, IF the `--timeout` duration expires before readiness is confirmed, THEN the SSM Port Forward CLI SHALL report failure with a descriptive error indicating timeout.

**Rationale:**
Unbounded waits can hang callers indefinitely. The timeout provides a safety net for cases where the agent never sends a readiness signal or error.

**Verification:**
Test by starting a session with `-w` against a non-responsive agent and verifying the CLI exits with a timeout error after the configured duration.

---

### Flag Message Filtering

**READY-005:** Event Driven

**Requirement:**
WHEN a `ConnectToPortError` flag message is received from the agent, the SSM Port Forward CLI SHALL NOT write raw flag bytes to the output stream.

**Rationale:**
Flag messages are control signals, not data. Writing their raw binary content to stdout or a TCP connection corrupts the data stream and confuses the connected client.

**Verification:**
Test by sending a flag message through the data channel and verifying zero bytes are written to the output stream.

---

### Session Termination on Port Error

**READY-006:** Event Driven

**Requirement:**
WHEN a `ConnectToPortError` flag message is received from the agent, the SSM Port Forward CLI SHALL log the error and terminate the port forwarding session.

**Rationale:**
A `ConnectToPortError` indicates the tunnel cannot function. Continuing the session would leave the caller connected to a non-functional tunnel with no data flow.

**Verification:**
Test by injecting a `ConnectToPortError` flag message and verifying the session is terminated and the error is logged.

---

### Positive Readiness Signal

**READY-007:** Event Driven

**Requirement:**
WHEN the agent sends a `StartPublicationMessage`, the SSM Port Forward CLI SHALL treat this as positive confirmation that the remote port is ready for data transfer.

**Rationale:**
The `start_publication` message is the SSM protocol's signal that the agent is ready to accept and forward data. This provides positive detection of readiness rather than relying on absence of errors or time-based heuristics.

**Verification:**
Test by sending a `StartPublicationMessage` through the data channel and verifying that the readiness channel is signaled.

---

### Combined Wait Logic

**READY-008:** Complex Event Driven

**Requirement:**
WHEN the `-w` flag is set, the SSM Port Forward CLI SHALL wait for one of: `StartPublicationMessage` received (success), `ConnectToPortError` received (failure), OR `--timeout` expired (failure), whichever occurs first, before reporting the result.

**Rationale:**
The combined wait logic ensures positive detection of readiness via `start_publication`, fast failure via `ConnectToPortError`, and bounded execution via timeout. This replaces the previous approach of only checking whether the local TCP listener was accepting connections.

**Verification:**
Test each of the three outcomes independently: (1) `StartPublicationMessage` arrives before timeout, (2) `ConnectToPortError` arrives before timeout, (3) timeout expires with no signal.

---
