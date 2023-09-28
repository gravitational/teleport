import React from 'react';
import { Box, Text } from 'design';
import { Option } from 'shared/components/Select';

import { TraitLabel, TraitsCreator } from '../Traits';

import { EligibilityOrGrantRolesFieldSelectAndCreate } from './Shared';

type Props = {
  grant: Grant;
  setGrant(g: Grant): void;
  isDisabled: boolean;
  roleOptions: Option[];
};

export type Grant = {
  rolesToGrant: Option[];
  traitsToGrant: TraitLabel[];
};

export const GrantSection = ({
  grant,
  setGrant,
  isDisabled,
  roleOptions,
}: Props) => {
  return (
    <>
      <Text fontSize="18px" mb={4}>
        Permissions Granted to List Members
      </Text>
      <EligibilityOrGrantRolesFieldSelectAndCreate
        options={roleOptions}
        isDisabled={isDisabled}
        onChange={(roles: Option[]) =>
          setGrant({ ...grant, rolesToGrant: roles || [] })
        }
        selected={grant.rolesToGrant}
        editKind="Grants"
        optional={grant.traitsToGrant.length > 0}
      />
      <Box mb={3} mt={3}>
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
