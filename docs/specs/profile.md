# Connection Profiling Requirements

## Overview

This document specifies requirements for opt-in performance profiling of the ssm-port-forward connection process. Profiling emits structured timing data for each sequential phase of connection establishment, enabling identification of latency bottlenecks.

**System Name:** SSM Port Forward CLI
**Tag Prefix:** PROFILE
**Version:** 1.0
**Last Updated:** 2026-04-18

## Requirements

### Opt-in Activation

**PROFILE-001:** Event Driven

**Requirement:**
WHEN the environment variable `SSM_PROFILE` is set to `1` or `true`, the SSM Port Forward CLI SHALL enable connection profiling.

**Rationale:**
Profiling is a developer diagnostic tool. Opt-in via environment variable keeps the CLI flag namespace clean and ensures zero impact on normal usage.

**Verification:**
Test that `profile.New()` returns a non-nil Profiler when `SSM_PROFILE=1` is set.

---

### Phase Timing

**PROFILE-002:** Event Driven

**Requirement:**
WHEN profiling is enabled, the SSM Port Forward CLI SHALL record the wall-clock duration of each sequential connection phase: `aws_session`, `ssm_start_session`, `websocket_open`, `session_type_set`, `wait_local_port`, `wait_remote_ready`.

**Rationale:**
Each phase involves distinct network I/O or blocking waits. Per-phase timing identifies which step dominates total connection latency.

**Verification:**
Test that Begin/End calls record durations for each phase and that all phase names appear in the output.

---

### Structured Output

**PROFILE-003:** Event Driven

**Requirement:**
WHEN profiling is enabled AND the connection completes or fails, the SSM Port Forward CLI SHALL emit a single JSON object to stderr containing all recorded phase durations and a total duration.

**Rationale:**
JSON output to stderr allows programmatic analysis without interfering with the port/PID JSON written to stdout. A single object (not streaming) simplifies parsing.

**Verification:**
Test that Emit produces valid JSON with total_ms and a phases array, and that it is written to stderr.

---

### Zero Overhead When Disabled

**PROFILE-004:** Unwanted Behaviour

**Requirement:**
WHEN `SSM_PROFILE` is not set or set to any value other than `1` or `true`, the profiling infrastructure SHALL NOT call `time.Now()`, allocate memory, or perform any I/O.

**Rationale:**
Profiling must have zero cost in production. The nil-receiver pattern ensures all method calls on a nil Profiler are no-ops.

**Verification:**
Test that `profile.New()` returns nil when the env var is unset, and that all methods on a nil Profiler do not panic and return immediately.

---

### Failure Recording

**PROFILE-005:** Event Driven

**Requirement:**
WHEN profiling is enabled AND a phase fails with an error, the SSM Port Forward CLI SHALL record that phase with status `error`, the error message, and the elapsed time before failure, then emit the profile.

**Rationale:**
Connection failures are the primary reason to profile. Knowing which phase failed and how long it took before failing is essential for diagnosis.

**Verification:**
Test that EndWithError records the error message and marks the phase status as `error`.

---
