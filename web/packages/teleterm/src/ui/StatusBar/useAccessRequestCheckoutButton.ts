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

import { useWorkspaceServiceState } from 'teleterm/ui/services/workspacesService';

import { useAppContext } from '../appContextProvider';

export function useAccessRequestsButton() {
  const ctx = useAppContext();
  useWorkspaceServiceState();

  const workspaceAccessRequest =
    ctx.workspacesService.getActiveWorkspaceAccessRequestsService();

  function toggleAccessRequestBar() {
    if (!workspaceAccessRequest) {
      return;
    }
    return workspaceAccessRequest.toggleBar();
  }

  function isCollapsed() {
    if (!workspaceAccessRequest) {
      return true;
    }
    return workspaceAccessRequest.getCollapsed();
  }

  function getAddedItemsCount() {
    if (!workspaceAccessRequest) {
      return 0;
    }
    return workspaceAccessRequest.getAddedItemsCount();
  }

  return {
    isCollapsed,
    toggleAccessRequestBar,
    getAddedItemsCount,
  };
}
