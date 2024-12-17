import { Box, Text, Flex, H2 } from 'design';
import { Option } from 'shared/components/Select';
import { AllUserTraits } from 'teleport/services/user';
import { ToolTipInfo } from 'shared/components/ToolTip';

import { HybridUserOption } from '../Shared/Shared';
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
  setMembers(m: Members): void;
  members: Members;
  noAccess: boolean;
};

export type Members = {
  selectedRolesRequired: Option[];
  eligibleMembers: HybridUserOption[];
  selectedMembers: HybridUserOption[];
  traitLabels: TraitLabel[];
  traitLookup: AllUserTraits;
};

export const MembersSection = ({
  fetchRoleOptions,
  isDisabled,
  setMembers,
  members,
  noAccess,
}: Props) => {
  return (
    <>
      <Flex alignItems="center" mb={2}>
        <H2 mr={2}>Members (Optional)</H2>
        <ToolTipInfo
          children={
            <>
              List members will receive long-term access to roles and traits
              granted by this access list, and their membership will be reviewed
              by list owners in periodic reviews.
            </>
          }
        />
      </Flex>
      <Text mb={5}>
        If a member does not have all required roles and traits defined here,
        membership will have no effect. They will not be granted any additional
        roles or traits by the list.
      </Text>
      <EligibilityOrGrantRolesFieldSelectAndCreate
        editKind="Member"
        optional={true}
        loadOptions={fetchRoleOptions}
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
      <EligibleUsersFieldSelectAndCreate
        selected={members.selectedMembers || []}
        isDisabled={isDisabled}
        onChange={vals =>
          setMembers({ ...members, selectedMembers: vals || [] })
        }
        options={members.eligibleMembers}
        label="Add Members (Optional)"
        noEligibleUsersFromNoAccess={noAccess}
      />
    </>
  );
};
