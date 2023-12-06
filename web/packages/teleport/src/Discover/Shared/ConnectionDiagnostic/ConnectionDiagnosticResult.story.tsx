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

import React from 'react';

import { ConnectionDiagnosticResult } from './ConnectionDiagnosticResult';

import type { Props } from './ConnectionDiagnosticResult';
import type { ConnectionDiagnosticTrace } from 'teleport/services/agents';

export default {
  title: 'Teleport/Discover/Shared/ConnectionDiagnostic',
};

export const Init = () => (
  <ConnectionDiagnosticResult {...props} diagnosis={null} />
);

export const NumberAndDescriptionOnSameLine = () => (
  <ConnectionDiagnosticResult
    {...props}
    numberAndDescriptionOnSameLine
    diagnosis={null}
  />
);

export const DiagnosisSuccess = () => (
  <ConnectionDiagnosticResult
    {...props}
    attempt={{ status: 'success' }}
    diagnosis={diagnosisSuccess}
  />
);

export const DiagnosisFailed = () => (
  <ConnectionDiagnosticResult
    {...props}
    attempt={{ status: 'success' }}
    diagnosis={diagnosisFailed}
  />
);

export const DiagnosisLoading = () => (
  <ConnectionDiagnosticResult {...props} attempt={{ status: 'processing' }} />
);

export const NoAccess = () => (
  <ConnectionDiagnosticResult {...props} canTestConnection={false} />
);

export const Error = () => (
  <ConnectionDiagnosticResult
    {...props}
    attempt={{ status: 'failed', statusText: 'some error message' }}
  />
);

const diagnosisSuccess = {
  id: 'id',
  labels: [],
  success: true,
  message: 'some diagnosis message',
  traces: [
    {
      traceType: 'rbac node',
      status: 'success',
      details: 'Resource exists.',
    },
    {
      traceType: 'network connectivity',
      status: 'success',
      details: 'Host is alive and reachable.',
    },
    {
      traceType: 'rbac principal',
      status: 'success',
      details: 'Successfully authenticated.',
    },
    {
      traceType: 'node ssh server',
      status: 'success',
      details: 'Established an SSH connection.',
    },
    {
      traceType: 'node ssh session',
      status: 'success',
      details: 'Created an SSH session.',
    },
    {
      traceType: 'node principal',
      status: 'success',
      details: 'User exists message.',
    },
  ] as ConnectionDiagnosticTrace[],
};

const diagnosisFailed = {
  id: 'id',
  labels: [],
  success: false,
  message: 'some diagnosis message',
  traces: [
    {
      traceType: 'rbac node',
      status: 'success',
      details: 'Resource exists.',
    },
    {
      traceType: 'network connectivity',
      status: 'success',
      details: 'Host is alive and reachable.',
    },
    {
      traceType: 'rbac principal',
      status: 'failed',
      details: 'Why rbac principal check failed',
      error: 'Some extra error log',
    },
    {
      traceType: 'node ssh session',
      status: 'failed',
      details: 'Why node ssh session might have failed',
      error: 'Some extra error log 2',
    },
  ] as ConnectionDiagnosticTrace[],
};

const props: Props = {
  attempt: { status: '' },
  diagnosis: null,
  canTestConnection: true,
  testConnection: () => null,
  stepNumber: 2,
  stepDescription: 'Verify that your example database is accessible',
};
