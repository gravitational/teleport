import { useCallback, useMemo } from 'react';

import { useAsync } from 'shared/hooks/useAsync';

import { AccessGraphDiff, AccessPathDiff } from 'e-teleport/AccessGraph/Diff';
import { accessGraphService } from 'e-teleport/services/accessgraph';
import cfg from 'teleport/config';
import { RolesContainer as Roles } from 'teleport/Roles';
import { unableToUpdatePreviewMessage } from 'teleport/Roles/RoleEditor/Shared';
import { Role } from 'teleport/services/resources';
import { storageService } from 'teleport/services/storageService';

const emptyDiff: AccessPathDiff = {
  base: { nodes: [], edges: [] },
  diff: [],
  change_id: '',
};

export const RolesE = () => {
  const roleTesterEnabled =
    cfg.isPolicyEnabled &&
    cfg.isPolicyRoleVisualizerEnabled &&
    storageService.getAccessGraphRoleTesterEnabled();

  const [roleDiffAttempt, updateRoleDiff] = useAsync(
    useCallback(async (role: Role) => {
      try {
        return await accessGraphService.getRoleDiff(role);
      } catch (err) {
        throw new Error(unableToUpdatePreviewMessage, { cause: err });
      }
    }, [])
  );

  const roleDiffProps = useMemo(() => {
    if (!roleTesterEnabled) {
      return undefined;
    }
    return {
      roleDiffElement: (
        <AccessGraphDiff
          diff={roleDiffAttempt.data ?? emptyDiff}
          loading={roleDiffAttempt.status === 'processing'}
        />
      ),
      roleDiffAttempt,
      updateRoleDiff,
    };
  }, [roleDiffAttempt, roleTesterEnabled, updateRoleDiff]);

  return <Roles roleDiffProps={roleDiffProps} />;
};
