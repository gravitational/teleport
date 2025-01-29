import { useCallback, useMemo } from 'react';

import { useAsync } from 'shared/hooks/useAsync';

import { AccessGraphDiff, AccessPathDiff } from 'e-teleport/AccessGraph/Diff';
import { accessGraphService } from 'e-teleport/services/accessgraph';
import cfg from 'teleport/config';
import { RolesContainer as Roles } from 'teleport/Roles';
import { storageService } from 'teleport/services/storageService';

const emptyDiff: AccessPathDiff = {
  base: { nodes: [], edges: [] },
  diff: [],
  change_id: '',
};

export const RolesE = () => {
  const roleTesterEnabled =
    cfg.isPolicyEnabled && storageService.getAccessGraphRoleTesterEnabled();

  const [roleDiffAttempt, updateRoleDiff] = useAsync(
    useCallback(accessGraphService.getRoleDiff, [])
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
      errorMessage: roleDiffAttempt.statusText,
      updateRoleDiff,
    };
  }, [roleDiffAttempt, roleTesterEnabled, updateRoleDiff]);

  return <Roles roleDiffProps={roleDiffProps} />;
};
