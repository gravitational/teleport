/*
 * Teleport
 * Copyright (C) 2024  Gravitational, Inc.
 *
 * This program is free software: you can redistribute it and/or modify
 * it under the terms of the GNU Affero General Public License as published by
 * the Free Software Foundation, either version 3 of the License, or
 * (at your option) any later version.
 *
 * This program is distributed in the hope that it will be useful,
 * but WITHOUT ANY WARRANTY; without even the implied warranty of
 * MERCHANTABILITY or FITNESS FOR A PARTICULAR PURPOSE.  See the
 * GNU Affero General Public License for more details.
 *
 * You should have received a copy of the GNU Affero General Public License
 * along with this program.  If not, see <http://www.gnu.org/licenses/>.
 */

package common

import (
	"bytes"
	"encoding/json"
	"fmt"
	"log/slog"
	"os/exec"

	"github.com/gravitational/trace"

	awsutils "github.com/gravitational/teleport/lib/utils/aws"
)

// awsCredentialProcessOutput defines the output required for
// credential_process in aws credentials profile.
//
// https://docs.aws.amazon.com/cli/v1/userguide/cli-configure-sourcing-external.html
type awsCredentialProcessOutput struct {
	Version         int    `json:"Version"`
	AccessKeyId     string `json:"AccessKeyId"`
	SecretAccessKey string `json:"SecretAccessKey"`
	SessionToken    string `json:"SessionToken"`
	Expiration      string `json:"Expiration"`
}

// onAWSAssumeRoleCredentials calls assume-role with AWS CLI and outputs the
// credentials in a format that credential_process (in AWS credentials profile)
// can consume.
// Note that credential_process does not cache external process credential.
func onAWSAssumeRoleCredentials(cf *CLIConf) error {
	awsApp, err := pickAWSApp(cf)
	if err != nil {
		return trace.Wrap(err)
	}

	args, err := makeAWSAssumeRoleArgs(awsApp.tc.Username, cf.AWSAssumeRole, awsApp.appInfo.AWSRoleARN)
	if err != nil {
		return trace.Wrap(err)
	}

	err = awsApp.StartLocalProxies(cf.Context)
	if err != nil {
		return trace.Wrap(err)
	}
	defer func() {
		if err := awsApp.Close(); err != nil {
			log.WithError(err).Error("Failed to close AWS app.")
		}
	}()

	slog.DebugContext(cf.Context, "Making aws sts assume-role call", "args", args)
	var output bytes.Buffer
	realStdout := cf.Stdout()
	cf.OverrideStdout = &output
	if err := awsApp.RunCommand(exec.Command(awsCLIBinaryName, args...)); err != nil {
		return trace.Wrap(err)
	}

	slog.DebugContext(cf.Context, "Output aws sts assume-role call", "output", string(output.Bytes()))
	credentialProcessOutput, err := makeCredentialProcessOutput(output.Bytes())
	if err != nil {
		return trace.Wrap(err)
	}
	fmt.Fprintln(realStdout, string(credentialProcessOutput))
	return nil
}

func makeAWSAssumeRoleARN(assumeRole, baseRoleARN string) (string, error) {
	// Already a role ARN.
	if _, err := awsutils.ParseRoleARN(assumeRole); err == nil {
		return assumeRole, nil
	}

	// Get partition and account ID from base role.
	arn, err := awsutils.ParseRoleARN(baseRoleARN)
	if err != nil {
		return "", trace.Wrap(err)
	}

	return awsutils.RoleARN(arn.Partition, arn.AccountID, assumeRole), nil
}

func makeAWSAssumeRoleArgs(teleportUser, assumeRole, baseRoleARN string) ([]string, error) {
	assumeRoleARN, err := makeAWSAssumeRoleARN(assumeRole, baseRoleARN)
	if err != nil {
		return nil, trace.Wrap(err)
	}

	// Use username as session name.
	sessionName := awsutils.MaybeHashRoleSessionName(teleportUser)

	return []string{"sts", "assume-role", "--role-arn", assumeRoleARN, "--role-session-name", sessionName}, nil
}

func makeCredentialProcessOutput(assumeRoleOutput []byte) ([]byte, error) {
	// "aws sts assume-role" provides most of the output we need except the
	// "Version":
	// https://docs.aws.amazon.com/cli/latest/reference/sts/assume-role.html
	var assumeRoleResponse struct {
		Credentials awsCredentialProcessOutput `json:"Credentials"`
	}
	if err := json.Unmarshal(assumeRoleOutput, &assumeRoleResponse); err != nil {
		return nil, trace.Wrap(err, "unmarshaling assume-role output")
	}

	assumeRoleResponse.Credentials.Version = 1
	credentialProcessOutput, err := json.Marshal(assumeRoleResponse.Credentials)
	if err != nil {
		return nil, trace.Wrap(err, "marshaling credential_process output")
	}
	return credentialProcessOutput, nil
}
