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

import { ResourceSearchError } from 'teleterm/ui/services/resources';
import { routing } from 'teleterm/ui/uri';

import { ResourceSearchErrors } from './ResourceSearchErrors';

export default {
  title: 'Teleterm/ModalsHost/ResourceSearchErrors',
};

export const Story = () => (
  <ResourceSearchErrors
    getClusterName={routing.parseClusterName}
    onCancel={() => {}}
    errors={[
      new ResourceSearchError(
        '/clusters/foo',
        new Error(
          '14 UNAVAILABLE: connection error: desc = "transport: authentication handshake failed: EOF"'
        )
      ),
      new ResourceSearchError(
        '/clusters/bar',
        new Error(
          '2 UNKNOWN: Unable to connect to ssh proxy at teleport.local:443. Confirm connectivity and availability.\n	dial tcp: lookup teleport.local: no such host'
        )
      ),
      new ResourceSearchError(
        '/clusters/baz',
        new Error(
          '14 UNAVAILABLE: connection error: desc = "transport: authentication handshake failed: EOF"'
        )
      ),
    ]}
  />
);
