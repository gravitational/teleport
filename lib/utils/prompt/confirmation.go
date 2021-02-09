/*
Copyright 2021 Gravitational, Inc.

Licensed under the Apache License, Version 2.0 (the "License");
you may not use this file except in compliance with the License.
You may obtain a copy of the License at

    http://www.apache.org/licenses/LICENSE-2.0

Unless required by applicable law or agreed to in writing, software
distributed under the License is distributed on an "AS IS" BASIS,
WITHOUT WARRANTIES OR CONDITIONS OF ANY KIND, either express or implied.
See the License for the specific language governing permissions and
limitations under the License.
*/

// Package prompt implements CLI prompts to the user.
//
// TODO(awly): mfa: support prompt cancellation (without losing data written
// after cancellation)
package prompt

import (
	"bufio"
	"fmt"
	"io"
	"strings"

	"github.com/gravitational/trace"
)

// Confirmation prompts the user for a yes/no confirmation for question.
// The prompt is written to out and the answer is read from in.
//
// question should be a plain sentece without "[yes/no]"-type hints at the end.
func Confirmation(out io.Writer, in io.Reader, question string) (bool, error) {
	fmt.Fprintf(out, "%s [y/N]: ", question)
	scan := bufio.NewScanner(in)
	if !scan.Scan() {
		return false, trace.WrapWithMessage(scan.Err(), "failed reading prompt response")
	}
	switch strings.ToLower(strings.TrimSpace(scan.Text())) {
	case "y", "yes":
		return true, nil
	default:
		return false, nil
	}
}

// PickOne prompts the user to pick one of the provided string options.
// The prompt is written to out and the answer is read from in.
//
// question should be a plain sentece without the list of provided options.
func PickOne(out io.Writer, in io.Reader, question string, options []string) (string, error) {
	fmt.Fprintf(out, "%s [%s]: ", question, strings.Join(options, ", "))
	scan := bufio.NewScanner(in)
	if !scan.Scan() {
		return "", trace.WrapWithMessage(scan.Err(), "failed reading prompt response")
	}
	answerOrig := scan.Text()
	answer := strings.ToLower(strings.TrimSpace(answerOrig))
	for _, opt := range options {
		if strings.ToLower(opt) == answer {
			return opt, nil
		}
	}
	return "", trace.BadParameter("%q is not a valid option, please specify one of [%s]", answerOrig, strings.Join(options, ", "))
}

// Input prompts the user for freeform text input.
// The prompt is written to out and the answer is read from in.
func Input(out io.Writer, in io.Reader, question string) (string, error) {
	fmt.Fprintf(out, "%s: ", question)
	scan := bufio.NewScanner(in)
	if !scan.Scan() {
		return "", trace.WrapWithMessage(scan.Err(), "failed reading prompt response")
	}
	return scan.Text(), nil
}
