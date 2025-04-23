/**
 * Teleport
 * Copyright (C) 2023  Gravitational, Inc.
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

import type {
  ConnectionDiagnostic,
  ConnectionDiagnosticTrace,
  ResourceLabel,
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

// makeLabelMapOfStrArrs converts an array of type ResourceLabel into
// a map of string arrays eg: {"os": ["mac", "linux"]} which is the
// data model expected in the backend for labels.
export function makeLabelMapOfStrArrs(labels: ResourceLabel[] = []) {
  const m: Record<string, string[]> = {};

  labels?.forEach(label => {
    if (!m[label.name]) {
      m[label.name] = [];
    }
    m[label.name].push(label.value);
  });

  return m;
}
