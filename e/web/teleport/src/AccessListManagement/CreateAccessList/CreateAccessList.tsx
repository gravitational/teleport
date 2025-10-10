import { Link } from 'react-router-dom';

import { Alert, Box, ButtonPrimary, ButtonSecondary, Flex, H1 } from 'design';
import { ArrowBack } from 'design/Icon';
import { Option } from 'shared/components/Select';
import Validation from 'shared/components/Validation';

import { AccessListManagementContextProvider } from 'e-teleport/AccessListManagement/AccessListManagementContext';
import cfg from 'e-teleport/config';
import {
  FeatureBox,
  FeatureHeader,
  FeatureHeaderTitle,
} from 'teleport/components/Layout';
import { AllUserTraits } from 'teleport/services/user';

import { NoAccessState } from '../NoAccessState';
import {
  FeatureLimitReached,
  featureLimitReachedBlurCss,
} from '../Shared/FeatureLimitReached';
import {
  HybridUserOption,
  matchRoles,
  matchTraits,
  UserOption,
} from '../Shared/Shared';
import {
  CreateAccessListContextProvider,
  useCreateAccessList,
} from './CreateAccessListContextProvider';
import { GrantSection } from './GrantSection';
import { MembersSection } from './MemberSection';
import { OwnersSection } from './OwnerSection';
import { SpecSection } from './SpecSection';

export const CreateAccessListWithProvider = () => (
  <AccessListManagementContextProvider>
    <CreateAccessListContextProvider>
      <CreateAccessList />
    </CreateAccessListContextProvider>
  </AccessListManagementContextProvider>
);

export function CreateAccessList() {
  return (
    <FeatureBox>
      <FeatureHeader alignItems="center" justifyContent="space-between">
        <FeatureHeaderTitle>
          <Flex alignItems="center">
            <ArrowBack
              as={Link}
              mr={2}
              size="large"
              color="text.main"
              to={cfg.getAccessListManagementRoute()}
            />
            <H1>Create a New Access List</H1>
          </Flex>
        </FeatureHeaderTitle>
      </FeatureHeader>

      <MainContent />
    </FeatureBox>
  );
}

const MainContent = () => {
  const {
    onCreate,
    memberGrant,
    setMemberGrant,
    ownerGrant,
    setOwnerGrant,
    createAttempt,
    featureLimitReached,
    canCreateAccessList,
  } = useCreateAccessList();

  if (!canCreateAccessList) {
    return <NoAccessState action="create" />;
  }

  return (
    <>
      {featureLimitReached && <FeatureLimitReached />}
      {createAttempt.status === 'failed' && (
        <Alert>{createAttempt.statusText}</Alert>
      )}
      <Validation>
        {({ validator }) => (
          <Box
            width="540px"
            style={featureLimitReached ? featureLimitReachedBlurCss : null}
          >
            <Box mb={8}>
              <SpecSection />
            </Box>
            <Box mb={5}>
              <GrantSection
                grant={memberGrant}
                setGrant={setMemberGrant}
                title="Permissions Granted to List Members"
                isOptional={true}
              />
            </Box>
            <Box mb={8}>
              <GrantSection
                grant={ownerGrant}
                setGrant={setOwnerGrant}
                title="Permissions Granted to List Owners"
                isOptional={true}
              />
            </Box>
            <Box mb={8}>
              <OwnersSection />
            </Box>
            <Box>
              <MembersSection />
            </Box>
            <Box mt={5} mb={8}>
              <ButtonPrimary
                onClick={() => onCreate(validator)}
                mr={3}
                disabled={createAttempt.status === 'processing'}
              >
                Create Access List
              </ButtonPrimary>
              <ButtonSecondary
                as={Link}
                mt={3}
                width="90 px"
                to={cfg.getAccessListManagementRoute()}
              >
                Cancel
              </ButtonSecondary>
            </Box>
          </Box>
        )}
      </Validation>
    </>
  );
};

// eligibleUsers returns users with roles and traits
// that match with the required roles and traits.
export function getEligibleUsers(
  rolesRequiredToBeEligible: Option[],
  requiredTraitsToBeEligible: AllUserTraits,
  users: UserOption[]
): UserOption[] {
  if (
    users.length === 0 ||
    (rolesRequiredToBeEligible.length === 0 &&
      Object.keys(requiredTraitsToBeEligible).length === 0)
  ) {
    return [];
  }

  const rolesRequired = rolesRequiredToBeEligible.map(opt => opt.value);
  let filteredUsers = matchRoles(rolesRequired, users);

  return matchTraits(requiredTraitsToBeEligible, filteredUsers);
}

// eligibleUsersAmongSelectedUsers checks if selected owners
// are still eligible and returns selected users who are found
// in the eligible list.
export function getEligibleUsersAmongSelectedUsers({
  eligibleUsers,
  selectedUsers,
}: {
  eligibleUsers: UserOption[];
  selectedUsers: HybridUserOption[];
}) {
  return selectedUsers.filter(selectedUser =>
    eligibleUsers.some(eligibleOwner => {
      if (typeof selectedUser.value === 'string') {
        return false;
      }
      return eligibleOwner.value.name === selectedUser.value.name;
    })
  );
}
