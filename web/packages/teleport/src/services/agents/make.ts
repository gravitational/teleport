/**
 * Copyright 2022 Gravitational, Inc.
 *
 * Licensed under the Apache License, Version 2.0 (the "License");
 * you may not use this file except in compliance with the License.
 * You may obtain a copy of the License at
 *
 *     http://www.apache.org/licenses/LICENSE-2.0
 *
 * Unless required by applicable law or agreed to in writing, software
 * distributed under the License is distributed on an "AS IS" BASIS,
 * WITHOUT WARRANTIES OR CONDITIONS OF ANY KIND, either express or implied.
 * See the License for the specific language governing permissions and
 * limitations under the License.
 */

import type {
  ConnectionDiagnostic,
  ConnectionDiagnosticTrace,
  AgentLabel,
} from './types';

export function makeConnectionDiagnostic(json: any): ConnectionDiagnostic {
  json = json || {};
  const { id, success, message, traces } = json;

  return {
    id,
    success,
    message,
    traces: makeTraces(traces),
  };
}

function makeTraces(traces: any): ConnectionDiagnosticTrace[] {
  if (!traces) {
    return [];
  }

  return traces.map(t => ({
    traceType: t.traceType,
    status: t.status?.toLowerCase(),
    details: t.details,
    error: t.error,
  }));
}

// makeLabelMapOfStrArrs converts an array of type AgentLabel into
// a map of string arrays eg: {"os": ["mac", "linux"]} which is the
// data model expected in the backend for labels.
export function makeLabelMapOfStrArrs(labels: AgentLabel[] = []) {
  const m: Record<string, string[]> = {};

  labels.forEach(label => {
    if (!m[label.name]) {
      m[label.name] = [];
    }
    m[label.name].push(label.value);
  });

  return m;
}
