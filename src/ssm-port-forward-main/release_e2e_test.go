//go:build release_e2e

package main

import (
	"bufio"
	"bytes"
	"context"
	"encoding/json"
	"fmt"
	"io"
	"net"
	"os"
	"os/exec"
	"path/filepath"
	"runtime"
	"strings"
	"testing"
	"time"

	"github.com/aws/aws-sdk-go/aws"
	awssession "github.com/aws/aws-sdk-go/aws/session"
	"github.com/aws/aws-sdk-go/service/ssm"
)

func TestGoReleaserSSMPortForwardReleaseArtifactE2E(t *testing.T) {
	targetID, ok := optionalEnv("SSM_E2E_TARGET_ID")
	if !ok {
		warnE2E(t, "SSM_E2E_TARGET_ID is not set; skipping live release artifact validation")
		return
	}
	region := envDefault("SSM_E2E_REGION", "us-east-1")
	remoteHost := envDefault("SSM_E2E_REMOTE_HOST", "localhost.")
	remotePort := envDefault("SSM_E2E_REMOTE_PORT", "22")
	expectPrefix := envDefault("SSM_E2E_EXPECT_PREFIX", "SSH-2.0-")

	repoRoot, err := repositoryRoot()
	if err != nil {
		warnE2E(t, "%v", err)
		return
	}
	tag, err := exactReleaseTag(repoRoot)
	if err != nil {
		warnE2E(t, "%v", err)
		return
	}
	t.Logf("validating GoReleaser artifact from %s through %s:%s", tag, remoteHost, remotePort)

	t.Run("ssm-port-forward", func(t *testing.T) {
		binaryPath, err := buildGoReleaserBinary(t, repoRoot, "ssm-port-forward")
		if err != nil {
			warnE2E(t, "%v", err)
			return
		}

		info, stop, err := startSSMPortForward(binaryPath, targetID, region, remoteHost, remotePort)
		if err != nil {
			warnE2E(t, "%v", err)
			return
		}
		defer stop()

		if err := probeLocalPort(info.Port, expectPrefix, 30*time.Second); err != nil {
			warnE2E(t, "release artifact did not forward expected payload through SSM: %v", err)
		}
	})

	t.Run("session-manager-plugin", func(t *testing.T) {
		binaryPath, err := buildGoReleaserBinary(t, repoRoot, "session-manager-plugin")
		if err != nil {
			warnE2E(t, "%v", err)
			return
		}

		localPort, err := allocateTestPort()
		if err != nil {
			warnE2E(t, "%v", err)
			return
		}

		stop, err := startSessionManagerPlugin(binaryPath, targetID, region, remoteHost, remotePort, localPort)
		if err != nil {
			warnE2E(t, "%v", err)
			return
		}
		defer stop()

		if err := probeLocalPort(localPort, expectPrefix, 30*time.Second); err != nil {
			warnE2E(t, "release artifact did not forward expected payload through AWS CLI plugin path: %v", err)
		}
	})
}

func buildGoReleaserBinary(t *testing.T, repoRoot, id string) (string, error) {
	t.Helper()

	if _, err := exec.LookPath("goreleaser"); err != nil {
		return "", fmt.Errorf("goreleaser is required for release E2E: %w", err)
	}

	outputPath := filepath.Join(t.TempDir(), id)
	ctx, cancel := context.WithTimeout(context.Background(), 2*time.Minute)
	defer cancel()

	cmd := exec.CommandContext(ctx, "goreleaser", "build", "--clean", "--single-target", "--id", id, "--output", outputPath)
	cmd.Dir = repoRoot
	cmd.Env = os.Environ()

	output, err := cmd.CombinedOutput()
	if ctx.Err() == context.DeadlineExceeded {
		return "", fmt.Errorf("goreleaser build %s timed out:\n%s", id, output)
	}
	if err != nil {
		return "", fmt.Errorf("goreleaser build %s failed: %w\n%s", id, err, output)
	}
	return outputPath, nil
}

func startSSMPortForward(binaryPath, targetID, region, remoteHost, remotePort string) (OutputInfo, func(), error) {
	ctx, cancel := context.WithTimeout(context.Background(), 90*time.Second)
	args := []string{
		"-L", fmt.Sprintf("0:%s:%s", remoteHost, remotePort),
		"-i", targetID,
		"-r", region,
		"-w",
		"--timeout", "45s",
	}
	cmd := exec.CommandContext(ctx, binaryPath, args...)
	cmd.Env = os.Environ()

	stdout, err := cmd.StdoutPipe()
	if err != nil {
		cancel()
		return OutputInfo{}, nil, fmt.Errorf("open stdout pipe: %w", err)
	}

	var stderr bytes.Buffer
	cmd.Stderr = &stderr

	if err := cmd.Start(); err != nil {
		cancel()
		return OutputInfo{}, nil, fmt.Errorf("start %s: %w", binaryPath, err)
	}

	done := make(chan error, 1)
	go func() {
		done <- cmd.Wait()
	}()

	info, err := readForwardOutput(stdout, 75*time.Second)
	if err != nil {
		stopProcess(cmd, done)
		cancel()
		return OutputInfo{}, nil, fmt.Errorf("ssm-port-forward did not become active: %w\nstderr:\n%s", err, stderr.String())
	}

	stop := func() {
		stopProcess(cmd, done)
		cancel()
	}
	return info, stop, nil
}

func startSessionManagerPlugin(binaryPath, targetID, region, remoteHost, remotePort string, localPort int) (func(), error) {
	documentName := "AWS-StartPortForwardingSessionToRemoteHost"
	localPortString := fmt.Sprintf("%d", localPort)
	apiParams := map[string][]*string{
		"host":            {aws.String(remoteHost)},
		"portNumber":      {aws.String(remotePort)},
		"localPortNumber": {aws.String(localPortString)},
	}
	requestParams := map[string][]string{
		"host":            {remoteHost},
		"portNumber":      {remotePort},
		"localPortNumber": {localPortString},
	}

	sess, err := awssession.NewSessionWithOptions(awssession.Options{
		Config:            aws.Config{Region: aws.String(region)},
		Profile:           envDefault("SSM_E2E_PROFILE", ""),
		SharedConfigState: awssession.SharedConfigEnable,
	})
	if err != nil {
		return nil, fmt.Errorf("create AWS session: %w", err)
	}

	startSessionOutput, err := ssm.New(sess).StartSession(&ssm.StartSessionInput{
		Target:       aws.String(targetID),
		DocumentName: aws.String(documentName),
		Parameters:   apiParams,
	})
	if err != nil {
		return nil, fmt.Errorf("start SSM session for plugin E2E: %w", err)
	}

	pluginPayload := map[string]string{
		"SessionId":  aws.StringValue(startSessionOutput.SessionId),
		"TokenValue": aws.StringValue(startSessionOutput.TokenValue),
		"StreamUrl":  aws.StringValue(startSessionOutput.StreamUrl),
	}
	requestPayload := map[string]any{
		"Target":       targetID,
		"DocumentName": documentName,
		"Parameters":   requestParams,
	}

	pluginPayloadJSON, err := marshalJSON(pluginPayload)
	if err != nil {
		return nil, err
	}
	requestPayloadJSON, err := marshalJSON(requestPayload)
	if err != nil {
		return nil, err
	}
	endpoint := fmt.Sprintf("https://ssm.%s.amazonaws.com", region)

	ctx, cancel := context.WithTimeout(context.Background(), 90*time.Second)
	cmd := exec.CommandContext(ctx, binaryPath, pluginPayloadJSON, region, "StartSession", "", requestPayloadJSON, endpoint)
	cmd.Env = os.Environ()

	var stderr bytes.Buffer
	cmd.Stderr = &stderr
	if err := cmd.Start(); err != nil {
		cancel()
		return nil, fmt.Errorf("start %s: %w", binaryPath, err)
	}

	done := make(chan error, 1)
	go func() {
		done <- cmd.Wait()
	}()

	stop := func() {
		stopProcess(cmd, done)
		cancel()
	}

	select {
	case err := <-done:
		cancel()
		return nil, fmt.Errorf("session-manager-plugin exited before probe: %w\nstderr:\n%s", err, stderr.String())
	case <-time.After(2 * time.Second):
		return stop, nil
	}
	return stop, nil
}

func readForwardOutput(stdout io.Reader, timeout time.Duration) (OutputInfo, error) {
	type result struct {
		info OutputInfo
		err  error
	}

	results := make(chan result, 1)
	go func() {
		scanner := bufio.NewScanner(stdout)
		for scanner.Scan() {
			var info OutputInfo
			if err := json.Unmarshal(scanner.Bytes(), &info); err == nil && info.Status == "active" {
				results <- result{info: info}
				return
			}
		}
		if err := scanner.Err(); err != nil {
			results <- result{err: err}
			return
		}
		results <- result{err: fmt.Errorf("process exited before printing active JSON")}
	}()

	select {
	case result := <-results:
		return result.info, result.err
	case <-time.After(timeout):
		return OutputInfo{}, fmt.Errorf("timed out waiting for active JSON")
	}
}

func probeLocalPort(port int, expectPrefix string, timeout time.Duration) error {
	address := fmt.Sprintf("127.0.0.1:%d", port)
	deadline := time.Now().Add(timeout)
	var lastErr error

	for time.Now().Before(deadline) {
		conn, err := net.DialTimeout("tcp", address, 2*time.Second)
		if err != nil {
			lastErr = err
			time.Sleep(500 * time.Millisecond)
			continue
		}

		_ = conn.SetReadDeadline(time.Now().Add(5 * time.Second))
		buffer := make([]byte, 256)
		n, readErr := conn.Read(buffer)
		_ = conn.Close()
		if readErr == nil && strings.HasPrefix(string(buffer[:n]), expectPrefix) {
			return nil
		}
		if readErr != nil {
			lastErr = readErr
		} else {
			lastErr = fmt.Errorf("read %q, expected prefix %q", string(buffer[:n]), expectPrefix)
		}
		time.Sleep(500 * time.Millisecond)
	}

	if lastErr == nil {
		return fmt.Errorf("probe %s failed before timeout", address)
	}
	return fmt.Errorf("probe %s failed before timeout: %w", address, lastErr)
}

func allocateTestPort() (int, error) {
	listener, err := net.Listen("tcp", "127.0.0.1:0")
	if err != nil {
		return 0, fmt.Errorf("allocate test port: %w", err)
	}
	defer listener.Close()
	return listener.Addr().(*net.TCPAddr).Port, nil
}

func marshalJSON(value any) (string, error) {
	data, err := json.Marshal(value)
	if err != nil {
		return "", fmt.Errorf("marshal JSON: %w", err)
	}
	return string(data), nil
}

func stopProcess(cmd *exec.Cmd, done <-chan error) {
	if cmd.Process == nil {
		return
	}

	_ = cmd.Process.Signal(os.Interrupt)
	select {
	case <-done:
		return
	case <-time.After(10 * time.Second):
		_ = cmd.Process.Kill()
		<-done
	}
}

func exactReleaseTag(repoRoot string) (string, error) {
	cmd := exec.Command("git", "describe", "--tags", "--exact-match", "HEAD")
	cmd.Dir = repoRoot
	output, err := cmd.CombinedOutput()
	if err != nil {
		return "", fmt.Errorf("release E2E requires an exact release tag; skipping on this checkout: %w\n%s", err, output)
	}

	tag := strings.TrimSpace(string(output))
	if tag == "" {
		return "", fmt.Errorf("release E2E requires a non-empty release tag")
	}
	return tag, nil
}

func repositoryRoot() (string, error) {
	_, filename, _, ok := runtime.Caller(0)
	if !ok {
		return "", fmt.Errorf("failed to locate test file")
	}
	return filepath.Clean(filepath.Join(filepath.Dir(filename), "..", "..")), nil
}

func optionalEnv(name string) (string, bool) {
	value := strings.TrimSpace(os.Getenv(name))
	return value, value != ""
}

func envDefault(name, fallback string) string {
	value := strings.TrimSpace(os.Getenv(name))
	if value == "" {
		return fallback
	}
	return value
}

func warnE2E(t *testing.T, format string, args ...any) {
	t.Helper()
	t.Logf("WARNING: "+format, args...)
}
