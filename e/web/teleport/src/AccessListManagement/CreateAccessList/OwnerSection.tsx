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

export const OwnersSection = () => {
  const { owners, setOwners, ownerGrant, setOwnerGrant, createAttempt } =
    useCreateAccessList();
  const isDisabled = createAttempt.status === 'processing';
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
      <Text mb={3}>
        If a Teleport user is assigned as an owner but does not have all
        required roles and traits defined in this section, ownership will have
        no effect. They will not be granted any additional roles or traits by
        the list.
      </Text>
      <EligibilityOrGrantRolesFieldSelectAndCreate
        userKind="Owners"
        rolesSelectedFor="eligibility"
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
        userKind="Owners"
        selectedMembers={owners.selectedOwners}
        setSelectedMembers={vals =>
          setOwners({ ...owners, selectedOwners: vals ?? [] })
        }
        attempt={createAttempt}
      />
      <GrantSection
        grant={ownerGrant}
        setGrant={setOwnerGrant}
        isOptional={true}
        userKind="Owners"
      />
    </>
  );
};
