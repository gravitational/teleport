import { Text, Box, Flex } from 'design';
import { Option } from 'shared/components/Select';
import { AllUserTraits } from 'teleport/services/user';
import { IconTooltip } from 'design/Tooltip';

import { H2 } from 'design';

import { HybridUserOption, UserOption } from '../Shared/Shared';
import {
  TraitLabel,
  TraitsCreator,
  convertTraitLabelsToAllUserTraits,
} from '../Traits';

import {
  EligibilityOrGrantRolesFieldSelectAndCreate,
  EligibleUsersFieldSelectAndCreate,
} from './Shared';

type Props = {
  fetchRoleOptions: (input: string) => Promise<Option[]>;
  isDisabled: boolean;
  setOwners(m: Owners): void;
  owners: Owners;
  noAccess: boolean;
};

export type Owners = {
  selectedRolesRequired: Option[];
  eligibleOwners: UserOption[];
  selectedOwners: HybridUserOption[];
  traitLabels: TraitLabel[];
  traitLookup: AllUserTraits;
};

export const OwnersSection = ({
  fetchRoleOptions,
  isDisabled,
  owners,
  setOwners,
  noAccess,
}: Props) => {
  return (
    <>
      <Flex alignItems="center" mb={2}>
        <H2 mr={2}>List Owners</H2>
        <IconTooltip
          children={
            <>
              List Owners are responsible for managing members and membership
              requirements for this access list, and must conduct periodic
              access reviews.
            </>
          }
        />
      </Flex>
      <Text mb={5}>
        If a Teleport user is assigned as an owner but does not have all
        required roles and traits defined in this section, ownership will have
        no effect.
      </Text>
      <EligibilityOrGrantRolesFieldSelectAndCreate
        editKind="Owner"
        loadOptions={fetchRoleOptions}
        isDisabled={isDisabled}
        onChange={(option: Option[]) =>
          setOwners({
            ...owners,
            selectedRolesRequired: option || [],
          })
        }
        selected={owners.selectedRolesRequired}
        optional={true}
      />
      <Box mb={3}>
        <TraitsCreator
          kind="Owner"
          traitLabels={owners.traitLabels}
          isDisabled={isDisabled}
          updateTraitLabels={(traitLabels: TraitLabel[]) =>
            setOwners({
              ...owners,
              traitLabels,
              traitLookup: convertTraitLabelsToAllUserTraits(traitLabels),
            })
          }
        />
      </Box>
      <EligibleUsersFieldSelectAndCreate
        selected={owners.selectedOwners || []}
        isDisabled={isDisabled}
        onChange={vals => setOwners({ ...owners, selectedOwners: vals || [] })}
        options={owners.eligibleOwners}
        label={'Add List Owners'}
        requiredErrMsg="List Owners are required"
        noEligibleUsersFromNoAccess={noAccess}
      />
    </>
  );
};
