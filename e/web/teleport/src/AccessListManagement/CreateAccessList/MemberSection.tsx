import { Box, Flex, H2, Text } from 'design';
import { IconTooltip } from 'design/Tooltip';
import { Option } from 'shared/components/Select';

import {
  convertTraitLabelsToAllUserTraits,
  TraitLabel,
  TraitsCreator,
} from '../Traits';
import { EnrollNewMembersFields } from '../ViewEditAccessList/Members/EnrollNewMembers';
import { useCreateAccessList } from './CreateAccessListContextProvider';
import { GrantSection } from './GrantSection';
import { EligibilityOrGrantRolesFieldSelectAndCreate } from './Shared';

export const MembersSection = () => {
  const { members, setMembers, memberGrant, setMemberGrant, createAttempt } =
    useCreateAccessList();
  const isDisabled = createAttempt.status === 'processing';

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
      <Text mb={3}>
        If a Teleport user is assigned as a member but does not have all
        required roles and traits defined in this section, membership will have
        no effect. They will not be granted any additional roles or traits by
        the list.
      </Text>
      <EligibilityOrGrantRolesFieldSelectAndCreate
        userKind="Members"
        rolesSelectedFor="eligibility"
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
        userKind="Members"
        optional
        selectedMembers={members.selectedMembers}
        setSelectedMembers={vals =>
          setMembers({ ...members, selectedMembers: vals ?? [] })
        }
        attempt={createAttempt}
      />
      <Box mb={5}>
        <GrantSection
          grant={memberGrant}
          setGrant={setMemberGrant}
          isOptional={true}
          userKind="Members"
        />
      </Box>
    </>
  );
};
