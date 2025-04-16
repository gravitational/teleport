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
        onDiscard={() => {}}
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
        onDiscard={() => {}}
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
        onDiscard={() => {}}
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
        onDiscard={() => {}}
      />
    </MockAppContextProvider>
  );
};
