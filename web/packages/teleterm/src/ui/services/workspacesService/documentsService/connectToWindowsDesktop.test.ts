/**
 * Teleport
 * Copyright (C) 2025 Gravitational, Inc.
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

import { makeRootCluster } from 'teleterm/services/tshd/testHelpers';
import { MockAppContext } from 'teleterm/ui/fixtures/mocks';
import { connectToWindowsDesktop } from 'teleterm/ui/services/workspacesService';

test('updates active workspace to match target desktop', async () => {
  const appContext = new MockAppContext();
  const clusterFoo = makeRootCluster({ uri: '/clusters/foo' });
  const clusterBar = makeRootCluster({ uri: '/clusters/bar' });
  appContext.addRootCluster(clusterFoo);
  // Sets "bar" as the active workspace.
  appContext.addRootCluster(clusterBar);
  expect(appContext.workspacesService.getRootClusterUri()).toBe(clusterBar.uri);

  await connectToWindowsDesktop(
    appContext,
    {
      uri: `${clusterFoo.uri}/windows_desktops/win`,
      login: 'admin',
    },
    { origin: 'connection_list' }
  );

  expect(appContext.workspacesService.getRootClusterUri()).toBe(clusterFoo.uri);
});
