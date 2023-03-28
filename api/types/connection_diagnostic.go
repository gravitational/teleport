/*
Copyright 2022 Gravitational, Inc.

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

package types

import (
	"github.com/gravitational/trace"

	"github.com/gravitational/teleport/api/utils"
)

const (
	// DiagnosticMessageSuccess is the message used when we the Connection was successful
	DiagnosticMessageSuccess = "success"

	// DiagnosticMessageFailed is the message used when we the Connection failed
	DiagnosticMessageFailed = "failed"
)

// ConnectionDiagnostic represents a Connection Diagnostic.
type ConnectionDiagnostic interface {
	// ResourceWithLabels provides common resource methods.
	ResourceWithLabels

	// Whether the connection was successful
	IsSuccess() bool
	// Sets the success flag
	SetSuccess(bool)

	// The underlying message
	GetMessage() string
	// Sets the undderlying message
	SetMessage(string)

	// The connection test traces
	GetTraces() []*ConnectionDiagnosticTrace

	// AppendTrace adds a trace to the ConnectionDiagnostic Traces
	AppendTrace(*ConnectionDiagnosticTrace)
}

type ConnectionsDiagnostic []ConnectionDiagnostic

var _ ConnectionDiagnostic = &ConnectionDiagnosticV1{}

// NewConnectionDiagnosticV1 creates a new ConnectionDiagnosticV1 resource.
func NewConnectionDiagnosticV1(name string, labels map[string]string, spec ConnectionDiagnosticSpecV1) (*ConnectionDiagnosticV1, error) {
	c := &ConnectionDiagnosticV1{
		ResourceHeader: ResourceHeader{
			Version: V1,
			Kind:    KindConnectionDiagnostic,
			Metadata: Metadata{
				Name:   name,
				Labels: labels,
			},
		},
		Spec: spec,
	}

	if err := c.CheckAndSetDefaults(); err != nil {
		return nil, trace.Wrap(err)
	}

	return c, nil
}

// CheckAndSetDefaults checks and sets default values for any missing fields.
func (c *ConnectionDiagnosticV1) CheckAndSetDefaults() error {
	if c.Spec.Message == "" {
		return trace.BadParameter("ConnectionDiagnosticV1.Spec missing Message field")
	}

	return nil
}

// GetLabel retrieves the label with the provided key. If not found
// value will be empty and ok will be false.
func (c *ConnectionDiagnosticV1) GetLabel(key string) (val string, ok bool) {
	v, ok := c.Metadata.Labels[key]
	return v, ok
}

// GetAllLabels returns combined static and dynamic labels.
func (c *ConnectionDiagnosticV1) GetAllLabels() map[string]string {
	return CombineLabels(c.Metadata.Labels, nil)
}

// GetStaticLabels returns the connection diagnostic static labels.
func (c *ConnectionDiagnosticV1) GetStaticLabels() map[string]string {
	return c.Metadata.Labels
}

// IsSuccess returns whether the connection was successful
func (c *ConnectionDiagnosticV1) IsSuccess() bool {
	return c.Spec.Success
}

// SetSuccess sets whether the Connection was a success or not
func (c *ConnectionDiagnosticV1) SetSuccess(b bool) {
	c.Spec.Success = b
}

// GetMessage returns the connection diagnostic message.
func (c *ConnectionDiagnosticV1) GetMessage() string {
	return c.Spec.Message
}

// SetMessage sets the summary message of the Connection Diagnostic
func (c *ConnectionDiagnosticV1) SetMessage(s string) {
	c.Spec.Message = s
}

// GetTraces returns the connection test traces
func (c *ConnectionDiagnosticV1) GetTraces() []*ConnectionDiagnosticTrace {
	return c.Spec.Traces
}

// AppendTrace adds a trace into the Traces list
func (c *ConnectionDiagnosticV1) AppendTrace(trace *ConnectionDiagnosticTrace) {
	c.Spec.Traces = append(c.Spec.Traces, trace)
}

// MatchSearch goes through select field values and tries to
// match against the list of search values.
func (c *ConnectionDiagnosticV1) MatchSearch(values []string) bool {
	fieldVals := append(utils.MapToStrings(c.GetAllLabels()), c.GetName())
	return MatchSearch(fieldVals, values, nil)
}

// Origin returns the origin value of the resource.
func (c *ConnectionDiagnosticV1) Origin() string {
	return c.Metadata.Labels[OriginLabel]
}

// SetOrigin sets the origin value of the resource.
func (c *ConnectionDiagnosticV1) SetOrigin(o string) {
	c.Metadata.Labels[OriginLabel] = o
}

// SetStaticLabels sets the connection diagnostic static labels.
func (c *ConnectionDiagnosticV1) SetStaticLabels(sl map[string]string) {
	c.Metadata.Labels = sl
}

// NewTraceDiagnosticConnection creates a new Connection Diagnostic Trace.
// If traceErr is not nil, it will set the Status to FAILED, SUCCESS otherwise.
func NewTraceDiagnosticConnection(traceType ConnectionDiagnosticTrace_TraceType, details string, traceErr error) *ConnectionDiagnosticTrace {
	ret := &ConnectionDiagnosticTrace{
		Status:  ConnectionDiagnosticTrace_SUCCESS,
		Type:    traceType,
		Details: details,
	}

	if traceErr != nil {
		ret.Status = ConnectionDiagnosticTrace_FAILED
		ret.Error = traceErr.Error()
	}

	return ret
}
