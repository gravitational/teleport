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
import { EligibilityOrGrantRolesFieldSelectAndCreate } from './Shared';

type Props = {
  grant: Grant;
  setGrant(g: Grant): void;
  isDisabled: boolean;
  fetchRoleOptions(input: string): Promise<Option[]>;
  title: string;
  isOptional?: boolean;
};

export type Grant = {
  rolesToGrant: Option[];
  traitsToGrant: TraitLabel[];
};

export const GrantSection = ({
  grant,
  setGrant,
  isDisabled,
  fetchRoleOptions,
  title,
  isOptional = false,
}: Props) => {
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
        loadOptions={fetchRoleOptions}
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
