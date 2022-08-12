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

import { TestConnection } from './TestConnection';

import type { ConnectionDiagnosticTrace } from 'teleport/services/agents';
import type { State } from './useTestConnection';

export default {
  title: 'Teleport/Discover/TestConnection',
};

export const LoadedInit = () => <TestConnection {...props} />;

export const Processing = () => (
  <TestConnection {...props} attempt={{ status: 'processing' }} />
);

export const LoadedWithDiagnosisSuccess = () => (
  <TestConnection {...props} diagnosis={mockDiagnosis} />
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
        details: 'Some failed detail.',
        error: 'ssh: handshake failed: EOF',
      } as ConnectionDiagnosticTrace,
    ],
  };
  return <TestConnection {...props} diagnosis={diagnosisWithErr} />;
};

export const Failed = () => (
  <TestConnection
    {...props}
    attempt={{ status: 'failed', statusText: 'some error message' }}
  />
);

const mockDiagnosis = {
  id: 'id',
  labels: [],
  success: true,
  message: 'some diagnosis message',
  traces: [
    {
      id: '',
      traceType: 'rbac node',
      status: 'success',
      details: 'Resource exists.',
    },
    {
      id: '',
      traceType: 'network connectivity',
      status: 'success',
      details: 'Host is alive and reachable.',
    },
    {
      id: '',
      traceType: 'rbac principal',
      status: 'success',
      details: 'Successfully authenticated.',
    },
    {
      id: '',
      traceType: 'node ssh server',
      status: 'success',
      details: 'Established an SSH connection.',
    },
    {
      id: '',
      traceType: 'node ssh session',
      status: 'success',
      details: 'Created an SSH session.',
    },
    {
      id: '',
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
  diagnosis: null,
};
