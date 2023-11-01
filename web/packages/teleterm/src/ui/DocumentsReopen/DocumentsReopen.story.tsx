/**
 * Copyright 2023 Gravitational, Inc
 *
 * Licensed under the Apache License, Version 2.0 (the "License");
 * you may not use this file except in compliance with the License.
 * You may obtain a copy of the License at
 *
 *      http://www.apache.org/licenses/LICENSE-2.0
 *
 * Unless required by applicable law or agreed to in writing, software
 * distributed under the License is distributed on an "AS IS" BASIS,
 * WITHOUT WARRANTIES OR CONDITIONS OF ANY KIND, either express or implied.
 * See the License for the specific language governing permissions and
 * limitations under the License.
 */

import React from 'react';

import { MockAppContextProvider } from 'teleterm/ui/fixtures/MockAppContextProvider';

import { DocumentsReopen } from './DocumentsReopen';

export default {
  title: 'Teleterm/ModalsHost/DocumentsReopen',
};

export const Story = () => {
  return (
    <MockAppContextProvider>
      <DocumentsReopen
        rootClusterUri="/clusters/foo.cloud.gravitational.io"
        numberOfDocuments={8}
        onConfirm={() => {}}
        onCancel={() => {}}
      />
    </MockAppContextProvider>
  );
};

export const OneTab = () => {
  return (
    <MockAppContextProvider>
      <DocumentsReopen
        rootClusterUri="/clusters/foo.cloud.gravitational.io"
        numberOfDocuments={1}
        onConfirm={() => {}}
        onCancel={() => {}}
      />
    </MockAppContextProvider>
  );
};

export const LongClusterName = () => {
  return (
    <MockAppContextProvider>
      <DocumentsReopen
        rootClusterUri="/clusters/foo.bar.baz.quux.cloud.gravitational.io"
        numberOfDocuments={42}
        onConfirm={() => {}}
        onCancel={() => {}}
      />
    </MockAppContextProvider>
  );
};

export const LongContinuousClusterName = () => {
  return (
    <MockAppContextProvider>
      <DocumentsReopen
        rootClusterUri={`/clusters/${Array(10)
          .fill(['foo', 'bar', 'baz', 'quux'], 0)
          .flat()
          .join('')}`}
        numberOfDocuments={680}
        onConfirm={() => {}}
        onCancel={() => {}}
      />
    </MockAppContextProvider>
  );
};
