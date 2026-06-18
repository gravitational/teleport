/*
 * Teleport
 * Copyright (C) 2025  Gravitational, Inc.
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

package moderation

import "context"

// CommandEvaluationRequest is a request to evaluate a single command against an
// AI moderation policy.
type CommandEvaluationRequest struct {
	SessionID   string
	Command     string
	Policy      string // natural-language policy from the role
	Model       string // name of an existing InferenceModel resource
	Participant string
	Login       string
	ServerID    string
	SessionKind string // "ssh" | "k8s"
}

// CommandEvaluationResult is the AI moderation decision.
type CommandEvaluationResult struct {
	Approved  bool
	Reasoning string
}

// CommandEvaluator evaluates a command against an AI moderation policy. The
// enterprise build registers an implementation backed by an InferenceModel;
// in OSS no evaluator is registered and callers fail closed.
type CommandEvaluator interface {
	EvaluateCommand(ctx context.Context, req CommandEvaluationRequest) (CommandEvaluationResult, error)
}
