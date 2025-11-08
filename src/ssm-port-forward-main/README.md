# SSM Port Forward

A command-line tool for AWS SSM port forwarding with SSH-like syntax and multi-hop support.

## Overview

`ssm-port-forward` provides a simple, SSH-style interface for AWS Systems Manager (SSM) port forwarding sessions. It wraps the session-manager-plugin to provide:

- SSH-like `-L` port forwarding syntax
- **Multi-hop forwarding** through bastion hosts (laptop -> bastion -> target)
- Port readiness validation with `--wait` flag
- Structured output (JSON) for automation
- Output to stdout or file for integration with other tools

## Installation

Build from source:
```bash
make build-local
# Binary will be at: bin/ssm-port-forward
```

Or use goreleaser:
```bash
make build
```

## Usage

### Basic Syntax

```bash
# Forward to localhost on bastion
ssm-port-forward -L localPort:remotePort --instance-id BASTION_ID [OPTIONS]

# Multi-hop: Forward through bastion to another host
ssm-port-forward -L localPort:remoteHost:remotePort --instance-id BASTION_ID [OPTIONS]
```

### Options

| Flag | Short | Description |
|------|-------|-------------|
| `-L` | | Port forward specification (localPort:[remoteHost:]remotePort) **[Required]** |
| `--instance-id` | `-i` | EC2 instance ID **[Required]** |
| `--region` | `-r` | AWS region |
| `--profile` | `-p` | AWS profile |
| `--document-name` | `-d` | SSM document name (default: auto-selected based on remote host) |
| `--output` | `-o` | Output file for port/PID info (default: stdout) |
| `--wait` | `-w` | Wait for port forward to be established |
| `--timeout` | | Timeout for port forward validation (default: 30s) |

### Examples

#### Basic port forwarding
```bash
# Forward local port 8080 to remote port 80
ssm-port-forward -L 8080:80 --instance-id i-1234567890abcdef0 --region us-east-1
```

#### Wait for connection to be established
```bash
# Wait for port forward to be ready before returning
ssm-port-forward -L 8080:80 -i i-1234567890abcdef0 -r us-east-1 --wait
```

#### Output to file for automation
```bash
# Save port and PID info to a file
ssm-port-forward -L 8080:80 -i i-1234567890abcdef0 -r us-east-1 -o /tmp/portfw.json
```

#### Use AWS profile
```bash
# Use a specific AWS profile
ssm-port-forward -L 3306:3306 -i i-abc123 -p prod-profile --wait
```

#### Multi-hop forwarding to database server
```bash
# Forward local 3306 to db-server:3306 through bastion
ssm-port-forward -L 3306:db-server.internal:3306 \
  -i i-bastion123 \
  -r us-east-1 \
  --wait
```

#### Database port forwarding with validation
```bash
# Forward to RDS through bastion instance
ssm-port-forward -L 5432:database.region.rds.amazonaws.com:5432 \
  -i i-bastion123 \
  -r us-east-1 \
  --wait \
  --timeout 60s
```

## Multi-Hop Port Forwarding

The tool supports multi-hop forwarding through a bastion host, similar to SSH's `-L` syntax.

### How it works

```
┌─────────┐       ┌─────────┐       ┌────────────┐
│  Laptop │ ────> │ Bastion │ ────> │ Target Host│
└─────────┘       └─────────┘       └────────────┘
   :3306             SSM            db-server:3306
```

**Example: Access database server through bastion**
```bash
ssm-port-forward -L 3306:db-server.internal:3306 -i i-bastion -r us-east-1 -w
```

This creates:
1. Local port 3306 on your laptop
2. SSM session to bastion instance (i-bastion)
3. Bastion forwards to db-server.internal:3306
4. You connect to localhost:3306 on your laptop

### Common Use Cases

#### Private RDS Database Access
```bash
# Forward to RDS endpoint through bastion
ssm-port-forward -L 5432:mydb.abc123.us-east-1.rds.amazonaws.com:5432 \
  -i i-bastion-instance \
  -r us-east-1 \
  --wait

# Now connect to the database
psql -h localhost -p 5432 -U dbuser mydb
```

#### Private Elasticsearch/OpenSearch
```bash
# Access ES cluster through bastion
ssm-port-forward -L 9200:vpc-my-domain.es.amazonaws.com:443 \
  -i i-bastion \
  -r us-east-1 \
  --wait

# Query ES
curl https://localhost:9200/_cluster/health
```

#### Multi-tier Application Access
```bash
# Access backend API server through bastion
ssm-port-forward -L 8080:api-server.internal:8080 \
  -i i-bastion \
  -r us-east-1 \
  --wait

# Access the API
curl http://localhost:8080/api/health
```

### Two-Part vs Three-Part Syntax

**Two-part** (localhost on bastion):
```bash
-L 8080:80  # Forwards to localhost:80 on the bastion itself
```

**Three-part** (multi-hop):
```bash
-L 8080:app-server:80  # Forwards through bastion to app-server:80
```

## Automatic Document Selection

The tool **automatically selects the correct SSM document** based on your port forwarding specification:

- **Two-part syntax** (`-L localPort:remotePort`): Uses `AWS-StartPortForwardingSession`
- **Three-part syntax** with remote host (`-L localPort:remoteHost:remotePort`): Automatically uses `AWS-StartPortForwardingSessionToRemoteHost`

You don't need to manually specify `--document-name` unless you want to override the default behavior.

### Examples

```bash
# Automatic: Uses AWS-StartPortForwardingSession
ssm-port-forward -L 8080:80 -i i-bastion -r us-east-1

# Automatic: Uses AWS-StartPortForwardingSessionToRemoteHost
ssm-port-forward -L 5432:rds.amazonaws.com:5432 -i i-bastion -r us-east-1 -w

# Manual override (if needed)
ssm-port-forward -L 8080:80 -i i-bastion -d AWS-StartPortForwardingSessionToRemoteHost -r us-east-1
```

## Dynamic Port Allocation (Port 0)

You can use `0` as the local port to let the operating system choose an available port automatically:

```bash
# OS will choose an available port
ssm-port-forward -L 0:3306 -i i-bastion -r us-east-1 -o /tmp/forward.json

# Check the allocated port from the output file
jq '.port' /tmp/forward.json
```

This is useful when:
- Running multiple port forwards simultaneously
- You don't care which local port is used
- Avoiding port conflicts in automated scripts

**Note**: When using port 0, check the output (stdout or file) to see which port was actually allocated.

## SSM Document Types

AWS SSM provides different document types for port forwarding scenarios. The tool automatically selects the appropriate one, but you can override with `--document-name` if needed.

### AWS-StartPortForwardingSession

Used for forwarding to localhost on the bastion host:
- Two-part syntax: `-L localPort:remotePort`
- Target is the bastion instance itself

```bash
# Forward to port 80 on the bastion itself
ssm-port-forward -L 8080:80 -i i-bastion -r us-east-1
```

### AWS-StartPortForwardingSessionToRemoteHost

Automatically used for multi-hop forwarding:
- Three-part syntax: `-L localPort:remoteHost:remotePort`
- Target is an RDS endpoint, EC2 instance, or other networked resource
- Bastion acts as a jump host

```bash
# Automatically uses AWS-StartPortForwardingSessionToRemoteHost
ssm-port-forward -L 5432:mydb.us-east-1.rds.amazonaws.com:5432 \
  -i i-bastion \
  -r us-east-1 -w
```

### Quick Reference

| Scenario | Document (Auto-Selected) | Example |
|----------|--------------------------|---------|
| Port on bastion | `AWS-StartPortForwardingSession` | `-L 8080:80` |
| Remote host via bastion | `AWS-StartPortForwardingSessionToRemoteHost` | `-L 5432:rds.amazonaws.com:5432` |
| OS-chosen port | Same as above | `-L 0:3306` |

## Output Format

The tool outputs JSON with connection information as a single line:

### Example output (stdout or file):
```json
{"type":"ssm-port-forward","port":8080,"pid":12345,"status":"active","timestamp":"2025-01-15T10:30:45Z","forwarding":"8080:80","bastion":"i-bastion123"}
```

### Fields:
- `type`: Output type identifier (always "ssm-port-forward")
- `port`: The local port that was opened (number)
- `pid`: Process ID of the ssm-port-forward process
- `status`: Connection status ("active")
- `timestamp`: When the connection was established (RFC3339 format)
- `forwarding`: The port forwarding specification (localPort:[remoteHost:]remotePort)
- `bastion`: The bastion instance ID

## Automation Examples

### Shell script integration
```bash
#!/bin/bash
# Start port forward and capture output
ssm-port-forward -L 8080:80 -i i-instance123 -r us-east-1 -w -o /tmp/pf.json

# Extract port from output
LOCAL_PORT=$(jq -r '.port' /tmp/pf.json)
PID=$(jq -r '.pid' /tmp/pf.json)

# Use the forwarded port
curl http://localhost:$LOCAL_PORT

# Cleanup when done
kill $PID
```

### Background process with validation
```bash
# Start port forward in background, wait for it to be ready
ssm-port-forward -L 3306:3306 -i i-db-bastion -r us-east-1 --wait &
PF_PID=$!

# Wait for the background process to complete validation
wait $PF_PID

# Now safe to connect
mysql -h 127.0.0.1 -P 3306 -u user -p database
```

### Docker entrypoint script
```bash
#!/bin/bash
# Establish port forward before starting application
ssm-port-forward -L 5432:rds.amazonaws.com:5432 \
  -i $BASTION_INSTANCE \
  -r $AWS_REGION \
  --wait \
  -o /tmp/pf.json &

# Start application that connects to localhost:5432
exec "$@"
```

## How It Works

1. **Parse arguments**: SSH-style `-L` syntax is parsed into local and remote ports
2. **Start SSM session**: Uses AWS SDK to call `StartSession` API with `AWS-StartPortForwardingSession` document
3. **Initialize data channel**: Establishes websocket connection to SSM service
4. **Port validation** (with `--wait`): Attempts TCP connection to verify port is listening
5. **Output information**: Writes JSON with port and PID information
6. **Maintain connection**: Keeps session alive until interrupted (if `--wait` is used)

## Requirements

- AWS credentials configured (via AWS CLI, environment variables, or IAM role)
- SSM permissions for target instance
- Instance must have SSM agent installed and running
- Network connectivity to AWS SSM endpoints

## Differences from AWS CLI

The standard AWS CLI approach:
```bash
aws ssm start-session \
  --target i-instance123 \
  --document-name AWS-StartPortForwardingSession \
  --parameters '{"portNumber":["80"],"localPortNumber":["8080"]}'
```

Benefits of `ssm-port-forward`:
- ✅ **Simpler syntax**: SSH-like `-L 8080:80` instead of JSON parameters
- ✅ **Port validation**: `--wait` flag ensures port is ready before proceeding
- ✅ **Structured output**: JSON output for automation
- ✅ **Better for scripts**: Easily parse port and PID information
- ✅ **Timeout handling**: Configurable timeout for connection validation

## Troubleshooting

### Port forward fails immediately
- Check AWS credentials and permissions
- Verify instance ID is correct and has SSM agent running
- Check security groups allow outbound HTTPS (443) to SSM endpoints

### Timeout waiting for port
- Increase `--timeout` duration
- Check that remote port is actually listening on the instance
- Verify no local firewall blocking the local port

### "Connection refused" on local port
- Don't use `--wait` if you need the process to stay in foreground
- The session-manager-plugin runs in a separate process

## License

Apache License 2.0 - See LICENSE file in repository root.
