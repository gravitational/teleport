import { Box, Flex, H2, Text } from 'design';
import { IconTooltip } from 'design/Tooltip';
import { Option } from 'shared/components/Select';
import { Attempt } from 'shared/hooks/useAttemptNext';

import { AllUserTraits } from 'teleport/services/user';

import { HybridUserOption, MemberSelection } from '../Shared/Shared';
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
  setMembers(m: Members): void;
  members: Members;
};

export type Members = {
  selectedRolesRequired: Option[];
  eligibleMembers: HybridUserOption[];
  selectedMembers: Option<MemberSelection>[];
  traitLabels: TraitLabel[];
  traitLookup: AllUserTraits;
};

export const MembersSection = ({
  attempt,
  isDisabled,
  setMembers,
  members,
}: Props) => {
  return (
    <>
      <Flex alignItems="center" mb={2}>
        <H2 mr={2}>Members (Optional)</H2>
        <IconTooltip>
          List members will receive long-term access to roles and traits granted
          by this access list, and their membership will be reviewed by list
          owners in periodic reviews.
        </IconTooltip>
      </Flex>
      <Text mb={5}>
        If a member does not have all required roles and traits defined here,
        membership will have no effect. They will not be granted any additional
        roles or traits by the list.
      </Text>
      <EligibilityOrGrantRolesFieldSelectAndCreate
        editKind="Member"
        optional={true}
        isDisabled={isDisabled}
        onChange={(option: Option[]) =>
          setMembers({
            ...members,
            selectedRolesRequired: option || [],
          })
        }
        selected={members.selectedRolesRequired}
      />
      <Box mb={3}>
        <TraitsCreator
          kind="Member"
          traitLabels={members.traitLabels}
          isDisabled={isDisabled}
          updateTraitLabels={(traitLabels: TraitLabel[]) =>
            setMembers({
              ...members,
              traitLabels,
              traitLookup: convertTraitLabelsToAllUserTraits(traitLabels),
            })
          }
        />
      </Box>
      <EnrollNewMembersFields
        optional
        selectedMembers={members.selectedMembers}
        setSelectedMembers={vals =>
          setMembers({ ...members, selectedMembers: vals ?? [] })
        }
        attempt={attempt}
      />
    </>
  );
};
