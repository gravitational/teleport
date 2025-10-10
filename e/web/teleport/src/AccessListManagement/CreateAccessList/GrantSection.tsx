import { useEffect, useMemo, useState } from 'react';

import { Alert, Box, H2 } from 'design';
import { Option } from 'shared/components/Select';
import useAttempt from 'shared/hooks/useAttemptNext';

import {
  fetchAndProcessSelectedRoles,
  rolesContainDenyRules,
} from 'e-teleport/AccessListManagement/Shared/Shared';
import type { Role } from 'teleport/services/resources';

import { TraitLabel, TraitsCreator } from '../Traits';
import { useCreateAccessList } from './CreateAccessListContextProvider';
import { EligibilityOrGrantRolesFieldSelectAndCreate } from './Shared';
import { Grant } from './types';

export const GrantSection = ({
  grant,
  setGrant,
  title,
  isOptional = false,
}: {
  grant: Grant;
  setGrant(g: Grant): void;
  title: string;
  isOptional?: boolean;
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
      <H2 mb={2}>{title}</H2>
      <EligibilityOrGrantRolesFieldSelectAndCreate
        isDisabled={isDisabled}
        onChange={(roles: Option[]) =>
          setGrant({ ...grant, rolesToGrant: roles || [] })
        }
        selected={grant.rolesToGrant}
        editKind="Grants"
        optional={grant.traitsToGrant.length > 0 || isOptional}
      />
      {selectedRolesContainDenyRules && (
        <Alert kind="warning" children={selectedRolesContainDenyRules} />
      )}
      <Box mb={3}>
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
