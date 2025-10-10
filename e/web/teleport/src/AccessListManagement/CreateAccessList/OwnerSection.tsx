import { Box, Flex, H2, Text } from 'design';
import { IconTooltip } from 'design/Tooltip';
import { Option } from 'shared/components/Select';
import { Attempt } from 'shared/hooks/useAttemptNext';

import { AllUserTraits } from 'teleport/services/user';

import { MemberSelection, UserOption } from '../Shared/Shared';
import {
  convertTraitLabelsToAllUserTraits,
  TraitLabel,
  TraitsCreator,
} from '../Traits';
import { EnrollNewMembersFields } from '../ViewEditAccessList/Members/EnrollNewMembers';
import { EligibilityOrGrantRolesFieldSelectAndCreate } from './Shared';

type Props = {
  attempt: Attempt;
  isDisabled: boolean;
  setOwners(m: Owners): void;
  owners: Owners;
};

export type Owners = {
  selectedRolesRequired: Option[];
  eligibleOwners: UserOption[];
  selectedOwners: Option<MemberSelection>[];
  traitLabels: TraitLabel[];
  traitLookup: AllUserTraits;
};

export const OwnersSection = ({
  attempt,
  isDisabled,
  owners,
  setOwners,
}: Props) => {
  return (
    <>
      <Flex alignItems="center" mb={2}>
        <H2 mr={2}>List Owners</H2>
        <IconTooltip>
          List Owners are responsible for managing members and membership
          requirements for this access list, and must conduct periodic access
          reviews.
        </IconTooltip>
      </Flex>
      <Text mb={5}>
        If a Teleport user is assigned as an owner but does not have all
        required roles and traits defined in this section, ownership will have
        no effect.
      </Text>
      <EligibilityOrGrantRolesFieldSelectAndCreate
        editKind="Owner"
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
      <EnrollNewMembersFields
        selectedMembers={owners.selectedOwners}
        setSelectedMembers={vals =>
          setOwners({ ...owners, selectedOwners: vals ?? [] })
        }
        attempt={attempt}
      />
    </>
  );
};
