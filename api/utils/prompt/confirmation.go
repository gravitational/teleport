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
package prompt

import (
	"context"
	"fmt"
	"io"
	urlpkg "net/url"
	"strings"

	"github.com/gravitational/trace"
)

// Reader is the interface for prompt readers.
type Reader interface {
	// ReadContext reads from the underlying buffer, respecting context
	// cancellation.
	ReadContext(ctx context.Context) ([]byte, error)
}

// SecureReader is the interface for password readers.
type SecureReader interface {
	// ReadPassword reads from the underlying buffer, respecting context
	// cancellation.
	ReadPassword(ctx context.Context) ([]byte, error)
}

// Confirmation prompts the user for a yes/no confirmation for question.
// The prompt is written to out and the answer is read from in.
//
// question should be a plain sentence without "[yes/no]"-type hints at the end.
//
// ctx can be canceled to abort the prompt.
func Confirmation(ctx context.Context, out io.Writer, in Reader, question string) (bool, error) {
	fmt.Fprintf(out, "%s [y/N]: ", question)
	answer, err := in.ReadContext(ctx)
	if err != nil {
		return false, trace.WrapWithMessage(err, "failed reading prompt response")
	}
	switch strings.ToLower(strings.TrimSpace(string(answer))) {
	case "y", "yes":
		return true, nil
	default:
		return false, nil
	}
}

// PickOne prompts the user to pick one of the provided string options.
// The prompt is written to out and the answer is read from in.
//
// question should be a plain sentence without the list of provided options.
//
// ctx can be canceled to abort the prompt.
func PickOne(ctx context.Context, out io.Writer, in Reader, question string, options []string) (string, error) {
	fmt.Fprintf(out, "%s [%s]: ", question, strings.Join(options, ", "))
	answerOrig, err := in.ReadContext(ctx)
	if err != nil {
		return "", trace.Wrap(err, "failed reading prompt response")
	}
	answer := strings.ToLower(strings.TrimSpace(string(answerOrig)))
	for _, opt := range options {
		if strings.ToLower(opt) == answer {
			return opt, nil
		}
	}
	return "", trace.BadParameter(
		"%q is not a valid option, please specify one of [%s]", strings.TrimSpace(string(answerOrig)), strings.Join(options, ", "))
}

// Input prompts the user for freeform text input.
// The prompt is written to out and the answer is read from in.
//
// ctx can be canceled to abort the prompt.
func Input(ctx context.Context, out io.Writer, in Reader, question string) (string, error) {
	fmt.Fprintf(out, "%s: ", question)
	answer, err := in.ReadContext(ctx)
	if err != nil {
		return "", trace.Wrap(err, "failed reading prompt response")
	}
	return strings.TrimSpace(string(answer)), nil
}

// Password prompts the user for a password. The prompt is written to out and
// the answer is read from in.
// The in reader has to be a terminal.
func Password(ctx context.Context, out io.Writer, in SecureReader, question string) (string, error) {
	if question != "" {
		fmt.Fprintf(out, "%s:\n", question)
	}
	answer, err := in.ReadPassword(ctx)
	if err != nil {
		return "", trace.Wrap(err, "failed reading prompt response")
	}
	return string(answer), nil // passwords not trimmed
}

// URLOptions are options for the URL prompt.
type URLOptions func(*urlOptions)

type urlOptions struct {
	urlValidator func(*urlpkg.URL) error
}

// WithURLValidator sets a custom URL validator for the URL prompt.
func WithURLValidator(validator func(*urlpkg.URL) error) URLOptions {
	return func(opts *urlOptions) {
		opts.urlValidator = validator
	}
}

// URL prompts the user for a URL. The prompt is written to out and the answer.
func URL(ctx context.Context, out io.Writer, in Reader, question string, opts ...URLOptions) (string, error) {
	url, err := Input(ctx, out, in, question)
	if err != nil {
		return "", trace.Wrap(err, "failed reading prompt response")
	}

	url = strings.TrimSpace(url)

	u, err := urlpkg.Parse(url)
	if err != nil {
		return "", trace.Wrap(err, "invalid URL")
	}

	opt := &urlOptions{}
	for _, o := range opts {
		o(opt)
	}

	if opt.urlValidator != nil {
		if err := opt.urlValidator(u); err != nil {
			return "", trace.Wrap(err, "failed to verify url")
		}
	}

	return url, nil
}
