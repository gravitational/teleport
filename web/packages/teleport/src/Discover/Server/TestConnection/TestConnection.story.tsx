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

import React from 'react';
import { MemoryRouter } from 'react-router';

import { TestConnection } from './TestConnection';

import type { ConnectionDiagnosticTrace } from 'teleport/services/agents';
import type { State } from './useTestConnection';

export default {
  title: 'Teleport/Discover/Server/TestConnection',
};

export const LoadedInit = () => (
  <MemoryRouter>
    <TestConnection {...props} />
  </MemoryRouter>
);

export const Processing = () => (
  <MemoryRouter>
    <TestConnection {...props} attempt={{ status: 'processing' }} />
  </MemoryRouter>
);

export const LoadedWithDiagnosisSuccess = () => (
  <MemoryRouter>
    <TestConnection {...props} diagnosis={mockDiagnosis} />
  </MemoryRouter>
);

export const LoadedWithDiagnosisFailure = () => {
  const diagnosisWithErr = {
    ...mockDiagnosis,
    success: false,
    traces: [
      ...mockDiagnosis.traces,
      {
        id: '',
        traceType: 'some trace type',
        status: 'failed',
        details:
          'Invalid user. Please ensure the principal "debian" is a valid Linux login in the target node. Output from Node: Failed to launch: user: unknown user debian.',
        error: 'ssh: handshake failed: EOF',
      } as ConnectionDiagnosticTrace,
      {
        id: '',
        traceType: 'some trace type',
        status: 'failed',
        details: 'Another error',
        error: 'some other error',
      } as ConnectionDiagnosticTrace,
    ],
  };
  return (
    <MemoryRouter>
      <TestConnection {...props} diagnosis={diagnosisWithErr} />
    </MemoryRouter>
  );
};

export const LoadedNoPerm = () => (
  <MemoryRouter>
    <TestConnection {...props} canTestConnection={false} />
  </MemoryRouter>
);

export const Failed = () => (
  <MemoryRouter>
    <TestConnection
      {...props}
      attempt={{ status: 'failed', statusText: 'some error message' }}
    />
  </MemoryRouter>
);

const mockDiagnosis = {
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

const props: State = {
  attempt: {
    status: 'success',
    statusText: '',
  },
  logins: ['root', 'llama', 'george_washington_really_long_name_testing'],
  startSshSession: () => null,
  runConnectionDiagnostic: () => null,
  nextStep: () => null,
  prevStep: () => null,
  diagnosis: null,
  canTestConnection: true,
};
