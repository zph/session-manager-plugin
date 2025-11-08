// Copyright 2021 Amazon.com, Inc. or its affiliates. All Rights Reserved.
// SPDX-License-Identifier: Apache-2.0

// Package version contains CLI version constant and utilities.

package version

// Version is the version of the CLI.
// This is set via ldflags during build: -ldflags "-X github.com/zph/session-manager-plugin/src/version.Version=x.y.z"
var Version = "dev"

// GitCommit is the git commit hash (with -dirty suffix if uncommitted changes exist).
// This is set via ldflags during build: -ldflags "-X github.com/zph/session-manager-plugin/src/version.GitCommit=abc123-dirty"
var GitCommit = "unknown"
