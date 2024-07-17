import React from 'react';
import { Box } from 'design';
import { Option } from 'shared/components/Select';

import { H2 } from 'design';

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
