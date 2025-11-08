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

// Package main represents the entry point of the ssm-port-forward CLI.
// This binary provides SSH-like port forwarding syntax for AWS SSM sessions.
package main

import (
	"encoding/json"
	"errors"
	"flag"
	"fmt"
	"net"
	"os"
	"os/signal"
	"strconv"
	"strings"
	"syscall"
	"time"

	"github.com/aws/aws-sdk-go/service/ssm"
	"github.com/zph/session-manager-plugin/src/datachannel"
	"github.com/zph/session-manager-plugin/src/log"
	"github.com/zph/session-manager-plugin/src/sdkutil"
	"github.com/zph/session-manager-plugin/src/sessionmanagerplugin/session"
	_ "github.com/zph/session-manager-plugin/src/sessionmanagerplugin/session/portsession"
	"github.com/google/uuid"
)

const (
	DefaultDocumentName = "AWS-StartPortForwardingSession"
)

type PortForwardConfig struct {
	LocalPort    string
	RemoteHost   string // Target host from bastion (default: localhost)
	RemotePort   string
	InstanceID   string
	Region       string
	Profile      string
	DocumentName string
	OutputFile   string
	Wait         bool
	Timeout      time.Duration
}

type OutputInfo struct {
	Type       string `json:"type"`
	Port       int    `json:"port"`
	PID        int    `json:"pid"`
	Status     string `json:"status"`
	Timestamp  string `json:"timestamp"`
	Forwarding string `json:"forwarding"`
	Bastion    string `json:"bastion"`
}

func main() {
	config, err := parseArgs()
	if err != nil {
		fmt.Fprintf(os.Stderr, "Error: %v\n", err)
		printUsage()
		os.Exit(1)
	}

	if err := run(config); err != nil {
		fmt.Fprintf(os.Stderr, "Error: %v\n", err)
		os.Exit(1)
	}
}

func parseArgs() (*PortForwardConfig, error) {
	config := &PortForwardConfig{}

	var localForward string
	flag.StringVar(&localForward, "L", "", "Local port forward specification (localPort:[remoteHost:]remotePort)")
	flag.StringVar(&config.InstanceID, "instance-id", "", "EC2 instance ID (bastion host)")
	flag.StringVar(&config.InstanceID, "i", "", "EC2 instance ID (short form)")
	flag.StringVar(&config.Region, "region", "", "AWS region")
	flag.StringVar(&config.Region, "r", "", "AWS region (short form)")
	flag.StringVar(&config.Profile, "profile", "", "AWS profile")
	flag.StringVar(&config.Profile, "p", "", "AWS profile (short form)")
	flag.StringVar(&config.DocumentName, "document-name", DefaultDocumentName, "SSM document name")
	flag.StringVar(&config.DocumentName, "d", DefaultDocumentName, "SSM document name (short form)")
	flag.StringVar(&config.OutputFile, "output", "", "Output file for port/PID info (default: stdout)")
	flag.StringVar(&config.OutputFile, "o", "", "Output file for port/PID info (short form)")
	flag.BoolVar(&config.Wait, "wait", false, "Wait for port forward to be established before exiting")
	flag.BoolVar(&config.Wait, "w", false, "Wait for port forward to be established (short form)")
	flag.DurationVar(&config.Timeout, "timeout", 30*time.Second, "Timeout for port forward validation")

	flag.Usage = printUsage
	flag.Parse()

	// Check for positional argument (non-flag) for -L style
	if localForward == "" && flag.NArg() > 0 {
		localForward = flag.Arg(0)
	}

	if localForward == "" {
		return nil, errors.New("port forward specification required (use -L localPort:[remoteHost:]remotePort)")
	}

	if config.InstanceID == "" {
		return nil, errors.New("instance-id is required")
	}

	// Parse local forward specification
	// Supports two formats:
	//   localPort:remotePort (forwards to localhost:remotePort on bastion)
	//   localPort:remoteHost:remotePort (forwards to remoteHost:remotePort from bastion)
	parts := strings.Split(localForward, ":")
	if len(parts) < 2 || len(parts) > 3 {
		return nil, fmt.Errorf("invalid port forward specification: %s (expected localPort:[remoteHost:]remotePort)", localForward)
	}

	config.LocalPort = parts[0]
	if len(parts) == 2 {
		// Format: localPort:remotePort (localhost implied)
		config.RemoteHost = "localhost"
		config.RemotePort = parts[1]
	} else {
		// Format: localPort:remoteHost:remotePort
		config.RemoteHost = parts[1]
		config.RemotePort = parts[2]
	}

	// Validate local port is a number (0 means OS will choose)
	if localPortNum, err := strconv.Atoi(config.LocalPort); err != nil {
		return nil, fmt.Errorf("invalid local port: %s", config.LocalPort)
	} else if localPortNum < 0 || localPortNum > 65535 {
		return nil, fmt.Errorf("local port out of range (0-65535): %s", config.LocalPort)
	}
	// Validate remote port is a number
	if remotePortNum, err := strconv.Atoi(config.RemotePort); err != nil {
		return nil, fmt.Errorf("invalid remote port: %s", config.RemotePort)
	} else if remotePortNum <= 0 || remotePortNum > 65535 {
		return nil, fmt.Errorf("remote port out of range (1-65535): %s", config.RemotePort)
	}

	// Auto-select document name if not explicitly specified and remote host is provided
	if config.DocumentName == DefaultDocumentName {
		if config.RemoteHost != "localhost" && config.RemoteHost != "127.0.0.1" {
			config.DocumentName = "AWS-StartPortForwardingSessionToRemoteHost"
		}
	}

	return config, nil
}

func printUsage() {
	fmt.Fprintf(os.Stderr, `Usage: ssm-port-forward [OPTIONS] -L localPort:[remoteHost:]remotePort

SSH-style port forwarding for AWS SSM sessions with multi-hop support.

Options:
  -L, --local-forward    Port forward specification
                         localPort:remotePort          (forward to localhost on bastion)
                         localPort:remoteHost:remotePort  (multi-hop through bastion)
  -i, --instance-id      EC2 instance ID (bastion host) (required)
  -r, --region           AWS region
  -p, --profile          AWS profile
  -d, --document-name    SSM document name (default: auto-selected based on remote host)
                         Auto-uses AWS-StartPortForwardingSessionToRemoteHost for remote hosts
  -o, --output           Output file for port/PID info (default: stdout)
  -w, --wait             Wait for port forward to be established
      --timeout          Timeout for port forward validation (default: 30s)

Examples:
  # Forward local port 8080 to port 80 on bastion
  ssm-port-forward -L 8080:80 --instance-id i-bastion123 --region us-east-1

  # Multi-hop: Forward local 3306 to server:3306 through bastion
  # (automatically uses AWS-StartPortForwardingSessionToRemoteHost)
  ssm-port-forward -L 3306:db-server.internal:3306 -i i-bastion123 -r us-east-1 -w

  # Database access through bastion to RDS
  # (automatically uses AWS-StartPortForwardingSessionToRemoteHost)
  ssm-port-forward -L 5432:mydb.xyz.rds.amazonaws.com:5432 -i i-bastion -r us-east-1 -w

  # Let OS choose local port (port 0)
  ssm-port-forward -L 0:80 -i i-bastion -r us-east-1 -w

  # Use AWS profile and output to file
  ssm-port-forward -L 3306:mysql-server:3306 -i i-bastion -p prod -o /tmp/db-forward.json

  # Simple localhost forward with validation
  ssm-port-forward -L 8080:8080 -i i-webserver -r us-east-1 -w

  # Use remote host document for multi-hop
  ssm-port-forward -L 5432:rds.amazonaws.com:5432 -i i-bastion \
    --document-name AWS-StartPortForwardingSessionToRemoteHost -r us-east-1 -w
`)
}

func run(config *PortForwardConfig) error {
	logger := log.Logger(true, "ssm-port-forward")

	// Set up signal handling
	sigChan := make(chan os.Signal, 1)
	signal.Notify(sigChan, syscall.SIGINT, syscall.SIGTERM)

	// Create SSM client
	sdkutil.SetRegionAndProfile(config.Region, config.Profile)
	sess, err := sdkutil.GetNewSessionWithEndpoint("")
	if err != nil {
		return fmt.Errorf("failed to create AWS session: %w", err)
	}
	ssmClient := ssm.New(sess)

	// If local port is 0, use OS to allocate an available port
	actualLocalPort := config.LocalPort
	if config.LocalPort == "0" {
		logger.Info("Local port 0 specified, allocating available port from OS...")
		allocatedPort, err := allocatePort()
		if err != nil {
			return fmt.Errorf("failed to allocate port: %w", err)
		}
		actualLocalPort = allocatedPort
		logger.Infof("OS allocated port: %s", actualLocalPort)
	}

	// Prepare port forwarding parameters
	params := map[string][]*string{
		"portNumber":      {&config.RemotePort},
		"localPortNumber": {&actualLocalPort},
	}

	// Add host parameter if not localhost (for multi-hop forwarding)
	if config.RemoteHost != "localhost" && config.RemoteHost != "127.0.0.1" {
		params["host"] = []*string{&config.RemoteHost}
	}

	// Start SSM session
	var forwardDesc string
	if config.RemoteHost == "localhost" || config.RemoteHost == "127.0.0.1" {
		forwardDesc = fmt.Sprintf("local %s -> bastion %s", config.LocalPort, config.RemotePort)
	} else {
		forwardDesc = fmt.Sprintf("local %s -> bastion -> %s:%s", config.LocalPort, config.RemoteHost, config.RemotePort)
	}
	logger.Infof("Starting port forward: %s on instance %s (document: %s)", forwardDesc, config.InstanceID, config.DocumentName)

	startSessionInput := &ssm.StartSessionInput{
		Target:       &config.InstanceID,
		DocumentName: &config.DocumentName,
		Parameters:   params,
	}

	startSessionOutput, err := ssmClient.StartSession(startSessionInput)
	if err != nil {
		return fmt.Errorf("failed to start SSM session: %w", err)
	}

	if startSessionOutput.SessionId == nil || startSessionOutput.TokenValue == nil || startSessionOutput.StreamUrl == nil {
		return errors.New("invalid session response: missing required fields")
	}

	logger.Infof("Session started: %s", *startSessionOutput.SessionId)

	// Create session
	clientId := uuid.NewString()
	sess2 := &session.Session{
		SessionId:   *startSessionOutput.SessionId,
		StreamUrl:   *startSessionOutput.StreamUrl,
		TokenValue:  *startSessionOutput.TokenValue,
		ClientId:    clientId,
		TargetId:    config.InstanceID,
		DataChannel: &datachannel.DataChannel{},
	}

	// Start session in goroutine
	sessionErr := make(chan error, 1)
	go func() {
		if err := sess2.Execute(logger); err != nil {
			sessionErr <- err
		}
	}()

	// Wait for port to be available if requested
	if config.Wait {
		logger.Infof("Waiting for port %s to be ready (timeout: %v)", actualLocalPort, config.Timeout)
		if err := waitForPort(actualLocalPort, config.Timeout); err != nil {
			return fmt.Errorf("port forward failed to establish: %w", err)
		}
		logger.Infof("Port forward established on local port %s", actualLocalPort)
	}

	// Construct forwarding specification with actual port
	var forwardingSpec string
	if config.RemoteHost == "localhost" || config.RemoteHost == "127.0.0.1" {
		forwardingSpec = fmt.Sprintf("%s:%s", actualLocalPort, config.RemotePort)
	} else {
		forwardingSpec = fmt.Sprintf("%s:%s:%s", actualLocalPort, config.RemoteHost, config.RemotePort)
	}

	// Convert port to integer for output
	portNum, err := strconv.Atoi(actualLocalPort)
	if err != nil {
		return fmt.Errorf("failed to convert port to integer: %w", err)
	}

	// Output port and PID info
	output := OutputInfo{
		Type:       "ssm-port-forward",
		Port:       portNum,
		PID:        os.Getpid(),
		Status:     "active",
		Timestamp:  time.Now().Format(time.RFC3339),
		Forwarding: forwardingSpec,
		Bastion:    config.InstanceID,
	}

	if err := writeOutput(config.OutputFile, output); err != nil {
		return fmt.Errorf("failed to write output: %w", err)
	}

	// If not waiting, exit now (port forward continues in background via session-manager-plugin)
	if !config.Wait {
		return nil
	}

	// Wait for signal or error
	select {
	case <-sigChan:
		logger.Info("Received interrupt signal, shutting down...")
		return nil
	case err := <-sessionErr:
		return fmt.Errorf("session error: %w", err)
	}
}

func waitForPort(port string, timeout time.Duration) error {
	deadline := time.Now().Add(timeout)
	for time.Now().Before(deadline) {
		conn, err := net.DialTimeout("tcp", "localhost:"+port, 100*time.Millisecond)
		if err == nil {
			conn.Close()
			return nil
		}
		time.Sleep(100 * time.Millisecond)
	}
	return fmt.Errorf("timeout waiting for port %s", port)
}

// allocatePort uses the OS to allocate an available port.
//
// RACE CONDITION WARNING: There is a known race condition between when we close
// the test listener and when SSM binds to the port. In the brief window between
// listener.Close() and SSM starting its listener, another process could grab the port.
//
// Mitigation approaches considered:
// - Keep listener open: Not possible - conflicts with SSM trying to bind
// - Let SSM allocate: Would require port detection (scanning), which adds complexity
// - Retry logic: Could be added if SSM session start fails due to port conflict
//
// In practice, the race window is very small (milliseconds) and the ephemeral port
// range is large (49152-65535), making collisions unlikely in normal operation.
func allocatePort() (string, error) {
	// Listen on port 0 to let OS choose an available port
	listener, err := net.Listen("tcp", "localhost:0")
	if err != nil {
		return "", fmt.Errorf("failed to allocate port: %w", err)
	}
	defer listener.Close()

	// Extract the port number from the listener's address
	addr := listener.Addr().(*net.TCPAddr)
	port := strconv.Itoa(addr.Port)

	return port, nil
}

func writeOutput(filename string, output OutputInfo) error {
	data, err := json.Marshal(output)
	if err != nil {
		return err
	}

	if filename == "" {
		fmt.Println(string(data))
		return nil
	}

	// Write as single line with newline at end for file
	data = append(data, '\n')
	return os.WriteFile(filename, data, 0644)
}

func stringPtr(s string) *string {
	return &s
}
