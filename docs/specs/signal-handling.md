# Signal Handling Specification for ssm-port-forward

## Overview
This specification defines signal handling requirements for rapid and graceful shutdown of the ssm-port-forward command.

## Requirements

### SIGNAL-001
WHEN the ssm-port-forward process receives SIGINT, THEN the process SHALL initiate shutdown within 100 milliseconds.

### SIGNAL-002
WHEN the ssm-port-forward process receives SIGTERM, THEN the process SHALL initiate shutdown within 100 milliseconds.

### SIGNAL-003
WHEN the ssm-port-forward process receives SIGHUP, THEN the process SHALL initiate shutdown within 100 milliseconds.

### SIGNAL-004
WHEN shutdown is initiated, THEN the process SHALL close the SSM session data channel.

### SIGNAL-005
WHEN shutdown is initiated, THEN the process SHALL terminate the SSM session.

### SIGNAL-006
WHEN all cleanup tasks are complete, THEN the process SHALL exit with status code 0.

### SIGNAL-007
WHEN the process is running with --wait flag, THEN signal handling SHALL be active.

### SIGNAL-008
WHEN the process is running without --wait flag, THEN signal handling SHALL be active.

### SIGNAL-009
WHEN cleanup takes longer than 5 seconds, THEN the process SHALL force exit with status code 1.

### SIGNAL-010
WHEN a signal is received during session establishment, THEN the process SHALL abort and cleanup any partial state.

### SIGNAL-011
WHEN a signal is received while waitForReady is waiting for port readiness (Phase 1 or Phase 2), THEN waitForReady SHALL return promptly with an error so the process can initiate cleanup and exit.

## Implementation Notes

The signal handler MUST:
- Register for SIGINT, SIGTERM, and SIGHUP
- Use buffered channel to prevent signal loss
- Call DataChannel.Close() to close websocket
- Call DataChannel.EndSession() to mark session ended
- Implement timeout to prevent hanging indefinitely
- Log shutdown actions for debugging
