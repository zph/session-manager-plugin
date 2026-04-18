// Package profile provides opt-in connection profiling for ssm-port-forward.
// Activation: set SSM_PROFILE=1 or SSM_PROFILE=true.
// All methods are safe to call on a nil *Profiler (no-ops).
package profile

import (
	"encoding/json"
	"io"
	"os"
	"time"
)

// Phase identifies a sequential connection phase.
type Phase int

const (
	PhaseAWSSession      Phase = iota // AWS SDK session + credential loading
	PhaseSSMStartSession              // SSM StartSession API call
	PhaseWebSocketOpen                // WebSocket connect + TLS + datachannel open
	PhaseSessionTypeSet               // Handshake + session type determination
	PhaseWaitLocalPort                // Phase 1: local TCP port readiness polling
	PhaseWaitRemoteReady              // Phase 2: remote readiness grace period
)

var phaseNames = [...]string{
	"aws_session",
	"ssm_start_session",
	"websocket_open",
	"session_type_set",
	"wait_local_port",
	"wait_remote_ready",
}

func (p Phase) String() string {
	if int(p) < len(phaseNames) {
		return phaseNames[p]
	}
	return "unknown"
}

// phaseRecord stores timing for one completed phase.
type phaseRecord struct {
	Name       string `json:"name"`
	DurationMs int64  `json:"duration_ms"`
	Status     string `json:"status"`
	Error      string `json:"error,omitempty"`
}

// profileOutput is the top-level JSON structure.
// Exported for use in tests only.
type profileOutput struct {
	Profile struct {
		TotalMs int64         `json:"total_ms"`
		Phases  []phaseRecord `json:"phases"`
	} `json:"profile"`
}

// Profiler records phase timings. A nil *Profiler is valid; all methods are no-ops.
type Profiler struct {
	start  time.Time
	phases []phaseRecord
}

// New returns a *Profiler if SSM_PROFILE is "1" or "true", nil otherwise.
// PROFILE-001, PROFILE-004
func New() *Profiler {
	v := os.Getenv("SSM_PROFILE")
	if v != "1" && v != "true" {
		return nil
	}
	return &Profiler{start: time.Now()}
}

// Span represents a timed phase in progress.
type Span struct {
	profiler *Profiler
	phase    Phase
	start    time.Time
}

// Begin starts timing a phase. Safe to call on nil Profiler.
// PROFILE-002
func (p *Profiler) Begin(phase Phase) Span {
	if p == nil {
		return Span{}
	}
	return Span{profiler: p, phase: phase, start: time.Now()}
}

// End records the phase as completed successfully.
// PROFILE-002
func (s Span) End() {
	if s.profiler == nil {
		return
	}
	s.profiler.phases = append(s.profiler.phases, phaseRecord{
		Name:       s.phase.String(),
		DurationMs: time.Since(s.start).Milliseconds(),
		Status:     "ok",
	})
}

// EndWithError records the phase as failed with the given error.
// PROFILE-005
func (s Span) EndWithError(err error) {
	if s.profiler == nil {
		return
	}
	rec := phaseRecord{
		Name:       s.phase.String(),
		DurationMs: time.Since(s.start).Milliseconds(),
		Status:     "ok",
	}
	if err != nil {
		rec.Status = "error"
		rec.Error = err.Error()
	}
	s.profiler.phases = append(s.profiler.phases, rec)
}

// Emit writes the profile as a single JSON line to w.
// PROFILE-003
func (p *Profiler) Emit(w io.Writer) {
	if p == nil {
		return
	}
	var out profileOutput
	out.Profile.TotalMs = time.Since(p.start).Milliseconds()
	out.Profile.Phases = p.phases
	data, err := json.Marshal(out)
	if err != nil {
		return
	}
	data = append(data, '\n')
	w.Write(data)
}
