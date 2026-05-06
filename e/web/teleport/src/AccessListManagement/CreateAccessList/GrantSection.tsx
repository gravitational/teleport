import { useEffect, useMemo, useState } from 'react';

import { Alert, Box } from 'design';
import { Option } from 'shared/components/Select';
import useAttempt from 'shared/hooks/useAttemptNext';

import {
  fetchAndProcessSelectedRoles,
  rolesContainDenyRules,
} from 'e-teleport/AccessListManagement/Shared/Shared';
import type { Role } from 'teleport/services/resources';

import { ScopedRoleGrantsEditor } from '../ScopedRoleGrantsEditor';
import { UserKind } from '../Shared/types';
import { TraitLabel, TraitsCreator } from '../Traits';
import { useCreateAccessList } from './CreateAccessListContextProvider';
import { EligibilityOrGrantRolesFieldSelectAndCreate } from './Shared';
import { Grant } from './types';

export const GrantSection = ({
  grant,
  setGrant,
  isOptional = false,
  userKind,
}: {
  grant: Grant;
  setGrant(g: Grant): void;
  isOptional?: boolean;
  userKind: UserKind;
}) => {
  const { createAttempt } = useCreateAccessList();
  const isDisabled = createAttempt.status === 'processing';

  const processedRolesFetchAttempt = useAttempt('');
  const [processedRoles, setProcessedRoles] = useState<Role[]>([]);

  useEffect(() => {
    if (processedRolesFetchAttempt.attempt.status === 'processing') {
      return;
    }
    if (!grant?.rolesToGrant?.length) {
      setProcessedRoles([]);
      return;
    }

    fetchAndProcessSelectedRoles(
      processedRolesFetchAttempt,
      grant?.rolesToGrant
    )
      .then(setProcessedRoles)
      .catch(() => setProcessedRoles([]));
  }, [grant?.rolesToGrant]);

  const selectedRolesContainDenyRules = useMemo(
    () => rolesContainDenyRules(processedRoles),
    [processedRoles]
  );

  return (
    <>
      <EligibilityOrGrantRolesFieldSelectAndCreate
        userKind={userKind}
        isDisabled={isDisabled}
        onChange={(roles: Option[]) =>
          setGrant({ ...grant, rolesToGrant: roles || [] })
        }
        selected={grant.rolesToGrant}
        rolesSelectedFor="grants"
        optional={grant.traitsToGrant.length > 0 || isOptional}
      />
      {selectedRolesContainDenyRules && (
        <Alert kind="warning" children={selectedRolesContainDenyRules} />
      )}
      <Box mt={-2} mb={4}>
        <Box mt={grant.scopedRolesToGrant.length > 0 ? 4 : 0}>
          <ScopedRoleGrantsEditor
            isDisabled={isDisabled}
            scopedRoleGrants={grant.scopedRolesToGrant}
            updateScopedRoleGrants={scopedRolesToGrant =>
              setGrant({ ...grant, scopedRolesToGrant })
            }
            userKind={userKind}
          />
        </Box>
        <TraitsCreator
          kind="Grants"
          traitLabels={grant.traitsToGrant}
          isDisabled={isDisabled}
          updateTraitLabels={(traitsToGrant: TraitLabel[]) =>
            setGrant({
              ...grant,
              traitsToGrant,
            })
          }
        />
      </Box>
    </>
  );
};
