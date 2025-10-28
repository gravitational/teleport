import { useCallback, useMemo, useRef } from 'react';

import { makeEmptyAttempt, useAsync } from 'shared/hooks/useAsync';

import { AccessGraphDiff, AccessPathDiff } from 'e-teleport/AccessGraph/Diff';
import { accessGraphService } from 'e-teleport/services/accessgraph';
import { unableToUpdatePreviewMessage } from 'teleport/Roles/RoleEditor/Shared';
import { RoleDiffProps } from 'teleport/Roles/Roles';
import { ApiError } from 'teleport/services/api/parseError';
import { Role } from 'teleport/services/resources';

import { useAccessGraphDemo } from './AccessGraphDemoContext';

const emptyDiff: AccessPathDiff = {
  base: { nodes: [], edges: [] },
  diff: [],
  change_id: '',
};

export const useRoleWithAccessGraph = () => {
  const abortControllerRef = useRef(new AbortController());
  const { isCloud, roleTesterEnabled, enableDemoMode, state, errorMessage } =
    useAccessGraphDemo();

  const [roleDiffAttempt, updateRoleDiff, updateAttempt] = useAsync(
    useCallback(async (role: Role) => {
      try {
        return await accessGraphService.getRoleDiff(
          role,
          abortControllerRef.current?.signal
        );
      } catch (err) {
        if (err instanceof ApiError && err.response.status === 404) {
          throw new Error(
            'Your graph is taking a few minutes to generate. After the initial generation, you will be able to preview access updates based on your role changes. Please try again in a few minutes.'
          );
        }
        throw new Error(unableToUpdatePreviewMessage, { cause: err });
      }
    }, [])
  );

  const clearRoleDiffAttempt = useCallback(() => {
    abortControllerRef.current?.abort();
    abortControllerRef.current = new AbortController();
    updateAttempt(makeEmptyAttempt());
  }, [updateAttempt]);

  const roleDiffProps: RoleDiffProps = useMemo(() => {
    if (!roleTesterEnabled && !isCloud) {
      return undefined;
    }
    return {
      roleDiffElement: (
        <AccessGraphDiff
          diff={roleDiffAttempt.data ?? emptyDiff}
          loading={roleDiffAttempt.status === 'processing'}
        />
      ),
      enableDemoMode,
      roleDiffAttempt,
      roleDiffState: state,
      roleDiffErrorMessage: errorMessage,
      updateRoleDiff,
      clearRoleDiffAttempt,
    };
  }, [
    enableDemoMode,
    isCloud,
    state,
    roleDiffAttempt,
    errorMessage,
    roleTesterEnabled,
    updateRoleDiff,
    clearRoleDiffAttempt,
  ]);

  return roleDiffProps;
};
