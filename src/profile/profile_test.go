package profile

import (
	"bytes"
	"encoding/json"
	"os"
	"testing"
	"time"
)

// PROFILE-004: WHEN SSM_PROFILE is not set,
// THEN New SHALL return nil.
func TestNewReturnsNilWhenDisabled(t *testing.T) {
	os.Unsetenv("SSM_PROFILE")
	p := New()
	if p != nil {
		t.Fatal("Expected nil profiler when SSM_PROFILE is unset")
	}
}

// PROFILE-001: WHEN SSM_PROFILE is set to "1",
// THEN New SHALL return a non-nil Profiler.
func TestNewReturnsProfilerWhenEnabled(t *testing.T) {
	t.Setenv("SSM_PROFILE", "1")
	p := New()
	if p == nil {
		t.Fatal("Expected non-nil profiler when SSM_PROFILE=1")
	}
}

// PROFILE-001: WHEN SSM_PROFILE is set to "true",
// THEN New SHALL return a non-nil Profiler.
func TestNewReturnsProfilerWhenEnabledTrue(t *testing.T) {
	t.Setenv("SSM_PROFILE", "true")
	p := New()
	if p == nil {
		t.Fatal("Expected non-nil profiler when SSM_PROFILE=true")
	}
}

// PROFILE-004: WHEN SSM_PROFILE is set to an unrecognized value,
// THEN New SHALL return nil.
func TestNewReturnsNilForUnrecognizedValue(t *testing.T) {
	t.Setenv("SSM_PROFILE", "yes")
	p := New()
	if p != nil {
		t.Fatal("Expected nil profiler for SSM_PROFILE=yes")
	}
}

// PROFILE-004: WHEN profiler is nil,
// THEN all methods SHALL be no-ops and not panic.
func TestNilProfilerMethodsAreNoOps(t *testing.T) {
	var p *Profiler
	// Must not panic
	span := p.Begin(PhaseAWSSession)
	span.End()
	span.EndWithError(nil)
	var buf bytes.Buffer
	p.Emit(&buf)
	if buf.Len() != 0 {
		t.Fatal("Expected no output from nil profiler")
	}
}

// PROFILE-002: WHEN Begin and End are called,
// THEN the phase duration SHALL be recorded.
func TestBeginEndRecordsDuration(t *testing.T) {
	t.Setenv("SSM_PROFILE", "1")
	p := New()

	span := p.Begin(PhaseAWSSession)
	time.Sleep(10 * time.Millisecond)
	span.End()

	var buf bytes.Buffer
	p.Emit(&buf)

	var result profileOutput
	if err := json.Unmarshal(buf.Bytes(), &result); err != nil {
		t.Fatalf("Failed to parse JSON: %v", err)
	}

	if len(result.Profile.Phases) != 1 {
		t.Fatalf("Expected 1 phase, got %d", len(result.Profile.Phases))
	}
	phase := result.Profile.Phases[0]
	if phase.Name != "aws_session" {
		t.Fatalf("Expected phase name 'aws_session', got '%s'", phase.Name)
	}
	if phase.DurationMs < 10 {
		t.Fatalf("Expected duration >= 10ms, got %d", phase.DurationMs)
	}
	if phase.Status != "ok" {
		t.Fatalf("Expected status 'ok', got '%s'", phase.Status)
	}
}

// PROFILE-005: WHEN EndWithError is called with an error,
// THEN the phase SHALL record status "error" and the error message.
func TestEndWithErrorRecordsFailure(t *testing.T) {
	t.Setenv("SSM_PROFILE", "1")
	p := New()

	span := p.Begin(PhaseSSMStartSession)
	span.EndWithError(os.ErrNotExist)

	var buf bytes.Buffer
	p.Emit(&buf)

	var result profileOutput
	if err := json.Unmarshal(buf.Bytes(), &result); err != nil {
		t.Fatalf("Failed to parse JSON: %v", err)
	}

	phase := result.Profile.Phases[0]
	if phase.Status != "error" {
		t.Fatalf("Expected status 'error', got '%s'", phase.Status)
	}
	if phase.Error == "" {
		t.Fatal("Expected non-empty error message")
	}
}

// PROFILE-003: WHEN Emit is called,
// THEN the output SHALL contain total_ms and a phases array.
func TestEmitOutputsJSON(t *testing.T) {
	t.Setenv("SSM_PROFILE", "1")
	p := New()

	p.Begin(PhaseAWSSession).End()
	p.Begin(PhaseSSMStartSession).End()

	var buf bytes.Buffer
	p.Emit(&buf)

	var result profileOutput
	if err := json.Unmarshal(buf.Bytes(), &result); err != nil {
		t.Fatalf("Failed to parse JSON: %v", err)
	}

	if result.Profile.TotalMs < 0 {
		t.Fatal("Expected non-negative total_ms")
	}
	if len(result.Profile.Phases) != 2 {
		t.Fatalf("Expected 2 phases, got %d", len(result.Profile.Phases))
	}
}

// PROFILE-002: Phase String() SHALL return snake_case names.
func TestPhaseStringNames(t *testing.T) {
	expected := map[Phase]string{
		PhaseAWSSession:       "aws_session",
		PhaseSSMStartSession:  "ssm_start_session",
		PhaseWebSocketOpen:    "websocket_open",
		PhaseSessionTypeSet:   "session_type_set",
		PhaseWaitLocalPort:    "wait_local_port",
		PhaseWaitRemoteReady:  "wait_remote_ready",
	}

	for phase, name := range expected {
		if phase.String() != name {
			t.Errorf("Phase %d: expected '%s', got '%s'", phase, name, phase.String())
		}
	}
}

// PROFILE-003: WHEN multiple phases are recorded,
// THEN total_ms SHALL be >= sum of individual durations.
func TestTotalMsCoversAllPhases(t *testing.T) {
	t.Setenv("SSM_PROFILE", "1")
	p := New()

	span1 := p.Begin(PhaseAWSSession)
	time.Sleep(10 * time.Millisecond)
	span1.End()

	span2 := p.Begin(PhaseSSMStartSession)
	time.Sleep(10 * time.Millisecond)
	span2.End()

	var buf bytes.Buffer
	p.Emit(&buf)

	var result profileOutput
	if err := json.Unmarshal(buf.Bytes(), &result); err != nil {
		t.Fatalf("Failed to parse JSON: %v", err)
	}

	var sumMs int64
	for _, ph := range result.Profile.Phases {
		sumMs += ph.DurationMs
	}
	if result.Profile.TotalMs < sumMs {
		t.Fatalf("total_ms (%d) < sum of phases (%d)", result.Profile.TotalMs, sumMs)
	}
}
