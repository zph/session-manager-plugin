// Copyright 2018 Amazon.com, Inc. or its affiliates. All Rights Reserved.
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

//go:build windows
// +build windows

// Package log is used to initialize logger
package log

import (
	"os"
)

const (
	// ApplicationFolder is the path under local app data.
	ApplicationFolderPrefix = "Amazon\\"
	LogsDirectory           = "Logs"
)

var EnvProgramFiles = os.Getenv("ProgramFiles") // Windows environment variable %ProgramFiles%

func getApplicationName(clientName string) string {
	var applicationName string
	if clientName == "ssmcli" {
		applicationName = "SSMCLI"
	} else if clientName == "session-manager-plugin" {
		applicationName = "SessionManagerPlugin"
	}

	return applicationName
}
