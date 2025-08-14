import React, { useEffect, useState } from 'react';
import { useHistory } from 'react-router';
import { Link } from 'react-router-dom';

import {
  Alert,
  Box,
  ButtonPrimary,
  ButtonSecondary,
  Flex,
  H1,
  Indicator,
} from 'design';
import { ArrowBack } from 'design/Icon';
import { Option } from 'shared/components/Select';
import Validation, { Validator } from 'shared/components/Validation';
import useAttempt from 'shared/hooks/useAttemptNext';

import {
  AccessListManagementContextProvider,
  useAccessListManagementContext,
} from 'e-teleport/AccessListManagement/AccessListManagementContext';
import cfg from 'e-teleport/config';
import {
  AccessListMemberKind,
  AccessListType,
  accessManagementService,
  convertReviewFrequencyIntoBackendParsableValue,
  ReviewDayOfMonth,
  ReviewFrequency,
} from 'e-teleport/services/accessmanagement';
import {
  FeatureBox,
  FeatureHeader,
  FeatureHeaderTitle,
} from 'teleport/components/Layout';
import { AllUserTraits } from 'teleport/services/user';
import useTeleport from 'teleport/useTeleport';

import { NoAccessState } from '../NoAccessState';
import { reviewDayOfMonthOpts, reviewFrequencyOpts } from '../Shared/Audit';
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
import { convertTraitLabelsToAllUserTraits } from '../Traits';
import { Grant, GrantSection } from './GrantSection';
import { Members, MembersSection } from './MemberSection';
import { Owners, OwnersSection } from './OwnerSection';
import { convertAccessListsToUserOptions } from './Shared';
import { Spec, SpecSection } from './SpecSection';

export const CreateAccessListWithProvider = () => (
  <AccessListManagementContextProvider>
    <CreateAccessList />
  </AccessListManagementContextProvider>
);

export function CreateAccessList() {
  const [featureLimitReached, setFeatureLimitReached] = useState(false);

  const {
    attempt: { attempt },
    accessLists,
    userOptions,
    usersAndRolesAttempt,
    fetchUsersAndRoles,
  } = useAccessListManagementContext();

  const { attempt: createAttempt, setAttempt: setCreateAttempt } =
    useAttempt('');

  const [owners, setOwners] = useState<Owners>({
    selectedRolesRequired: [],
    eligibleOwners: [],
    selectedOwners: [],
    traitLabels: [],
    traitLookup: {},
  });
  const [members, setMembers] = useState<Members>({
    selectedRolesRequired: [],
    eligibleMembers: [],
    selectedMembers: [],
    traitLabels: [],
    traitLookup: {},
  });

  // Fetch users and roles if not already fetched.
  useEffect(() => {
    if (usersAndRolesAttempt.status !== 'success') {
      fetchUsersAndRoles();
    }
    // eslint-disable-next-line react-hooks/exhaustive-deps
  }, []);

  useEffect(() => {
    if (
      cfg.oss.entitlements.AccessLists.enabled &&
      !cfg.oss.entitlements.AccessLists.limit
    ) {
      return;
    }

    if (
      attempt.status !== 'processing' &&
      accessLists.length >= cfg.oss.entitlements.AccessLists.limit
    ) {
      setFeatureLimitReached(true);
    }
    // eslint-disable-next-line react-hooks/exhaustive-deps
  }, [attempt.status]);

  // Update owners.
  useEffect(() => {
    let eligibleOwners: UserOption[] = userOptions;
    let selectedOwners: HybridUserOption[] = [];
    const rolesRequiredToBeEligible = owners.selectedRolesRequired;

    // Only filter for eligible owners if required roles or traits
    // are defined. Otherwise, all users are eligible.
    if (
      owners.selectedRolesRequired.length > 0 ||
      owners.traitLabels.length > 0
    ) {
      eligibleOwners = getEligibleUsers(
        rolesRequiredToBeEligible,
        owners.traitLookup,
        userOptions
      );
      if (eligibleOwners.length > 0 && owners.selectedOwners.length > 0) {
        selectedOwners = getEligibleUsersAmongSelectedUsers({
          eligibleUsers: eligibleOwners,
          selectedUsers: owners.selectedOwners,
        });
      }
    }

    eligibleOwners = eligibleOwners.concat(
      convertAccessListsToUserOptions(accessLists, owners.eligibleOwners)
    );

    setOwners({ ...owners, eligibleOwners, selectedOwners });
  }, [
    userOptions,
    owners.selectedRolesRequired,
    owners.traitLabels,
    accessLists,
  ]);

  // Update members.
  useEffect(() => {
    let eligibleMembers: UserOption[] = userOptions;
    let selectedMembers: HybridUserOption[] = [];
    const rolesRequiredToBeEligible = members.selectedRolesRequired;

    // Only filter for eligible members if required roles or traits
    // are defined. Otherwise, all users are eligible.
    if (
      members.selectedRolesRequired.length > 0 ||
      members.traitLabels.length > 0
    ) {
      eligibleMembers = getEligibleUsers(
        rolesRequiredToBeEligible,
        members.traitLookup,
        userOptions
      );

      if (eligibleMembers.length > 0 && members.selectedMembers.length > 0) {
        selectedMembers = getEligibleUsersAmongSelectedUsers({
          eligibleUsers: eligibleMembers,
          selectedUsers: members.selectedMembers,
        });
      }
    }

    eligibleMembers = eligibleMembers.concat(
      convertAccessListsToUserOptions(accessLists, members.eligibleMembers)
    );

    setMembers({ ...members, eligibleMembers, selectedMembers });
  }, [
    userOptions,
    members.selectedRolesRequired,
    members.traitLabels,
    accessLists,
  ]);

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

      <MainContent
        attempt={attempt}
        createAttempt={createAttempt}
        setCreateAttempt={setCreateAttempt}
        featureLimitReached={featureLimitReached}
        owners={owners}
        setOwners={setOwners}
        members={members}
        setMembers={setMembers}
      />
    </FeatureBox>
  );
}

const MainContent = ({
  attempt,
  createAttempt,
  setCreateAttempt,
  featureLimitReached,
  owners,
  setOwners,
  members,
  setMembers,
}: {
  attempt: ReturnType<
    typeof useAccessListManagementContext
  >['usersAndRolesAttempt'];
  createAttempt: ReturnType<typeof useAttempt>['attempt'];
  setCreateAttempt: ReturnType<typeof useAttempt>['setAttempt'];
  featureLimitReached: boolean;
  owners: Owners;
  setOwners: React.Dispatch<React.SetStateAction<Owners>>;
  members: Members;
  setMembers: React.Dispatch<React.SetStateAction<Members>>;
}) => {
  const ctx = useTeleport();
  const { fetchRoleOptions, userOptions } = useAccessListManagementContext();
  const history = useHistory();
  const perms = ctx.storeUser.getAccessListAccess();
  const canCreate = perms.create && perms.list && perms.read;
  const accessListCreator = ctx.storeUser.getUsername();

  const [spec, setSpec] = useState<Spec>(() => ({
    title: '',
    description: '',
    // Default first day of month.
    reviewDayOfMonth: reviewDayOfMonthOpts.find(o => {
      return o.value === ReviewDayOfMonth.FirstDayOfMonth;
    }),
    // Default to 6 months.
    reviewFrequency: reviewFrequencyOpts.find(
      o => o.value === ReviewFrequency.SixMonths
    ),
    auditStartDate: null,
  }));
  const [grant, setGrant] = useState<Grant>({
    rolesToGrant: [],
    traitsToGrant: [],
  });
  const [ownerGrant, setOwnerGrant] = useState<Grant>({
    rolesToGrant: [],
    traitsToGrant: [],
  });

  if (!canCreate) {
    return <NoAccessState action="create" />;
  }

  // Handle potential error states first.
  switch (attempt.status) {
    case 'processing':
      return (
        <Box textAlign="center" m={10}>
          <Indicator />
        </Box>
      );
    case 'failed':
      return <Alert children={attempt.statusText} />;
    case 'success':
      break;
    default:
      return null;
  }

  const handleOnCreate = (validator: Validator) => {
    if (!validator.validate()) {
      return;
    }

    // We don't need to setAttempt to "success"
    // since we are unmounting right after updating.
    setCreateAttempt({ status: 'processing' });

    const getNameAndMembershipKind = (o: HybridUserOption) => {
      if (typeof o.value !== 'object') {
        return { name: o.value, membership_kind: AccessListMemberKind.User };
      }

      return {
        name: o.value.name,
        membership_kind:
          'membershipKind' in o.value
            ? o.value.membershipKind
            : AccessListMemberKind.User,
      };
    };

    const listToCreate = {
      // specs
      type: AccessListType.Default,
      title: spec.title,
      description: spec.description,
      grants: {
        roles: grant.rolesToGrant.map(r => r.value),
        traits: convertTraitLabelsToAllUserTraits(grant.traitsToGrant),
      },
      owner_grants: {
        roles: ownerGrant.rolesToGrant.map(r => r.value),
        traits: convertTraitLabelsToAllUserTraits(ownerGrant.traitsToGrant),
      },
      audit: {
        recurrence: {
          frequency: convertReviewFrequencyIntoBackendParsableValue(
            spec.reviewFrequency.value
          ),
          day_of_month: spec.reviewDayOfMonth.value,
        },
        next_audit_date: spec.auditStartDate,
      },
      // owners
      ownership_requires: {
        roles: owners.selectedRolesRequired.map(r => r.value),
        traits: convertTraitLabelsToAllUserTraits(owners.traitLabels),
      },
      owners: owners.selectedOwners.map(o => getNameAndMembershipKind(o)),
      // members
      membership_requires: {
        roles: members.selectedRolesRequired.map(r => r.value),
        traits: convertTraitLabelsToAllUserTraits(members.traitLabels),
      },
      members: members.selectedMembers.map(m => {
        const { name, membership_kind } = getNameAndMembershipKind(m);
        return {
          name,
          joined: new Date(),
          added_by: accessListCreator,
          membership_kind,
        };
      }),
    };

    accessManagementService
      .createAccessList(listToCreate)
      // After creating, go back to access list listing.
      // Because of backend caching, we send the created list
      // as router state to be used to update the listing.
      .then(createdList => {
        // Add member counts to the created list in case they don't exist.
        if (!createdList.membersCount || !createdList.memberListCount) {
          const [membersCount, memberListCount] = listToCreate.members.reduce(
            (acc, m) => [
              acc[0] +
                (m.membership_kind === AccessListMemberKind.List ? 0 : 1),
              acc[1] +
                (m.membership_kind === AccessListMemberKind.List ? 1 : 0),
            ],
            [0, 0]
          );
          createdList.membersCount = membersCount;
          createdList.memberListCount = memberListCount;
        }
        history.push(cfg.getAccessListManagementRoute(), { createdList });
      })
      .catch((e: Error) =>
        setCreateAttempt({ status: 'failed', statusText: e.message })
      );
  };

  return (
    <>
      {featureLimitReached && <FeatureLimitReached />}
      {createAttempt.status === 'failed' && (
        <Alert children={createAttempt.statusText} />
      )}
      <Validation>
        {({ validator }) => (
          <Box
            width="540px"
            style={featureLimitReached ? featureLimitReachedBlurCss : null}
          >
            <Box mb={8}>
              <SpecSection
                spec={spec}
                setSpec={setSpec}
                isDisabled={
                  createAttempt.status === 'processing' || featureLimitReached
                }
              />
            </Box>
            <Box mb={5}>
              <GrantSection
                grant={grant}
                setGrant={setGrant}
                fetchRoleOptions={fetchRoleOptions}
                isDisabled={createAttempt.status === 'processing'}
                title="Permissions Granted to List Members"
                isOptional={true}
              />
            </Box>
            <Box mb={8}>
              <GrantSection
                grant={ownerGrant}
                setGrant={setOwnerGrant}
                fetchRoleOptions={fetchRoleOptions}
                isDisabled={createAttempt.status === 'processing'}
                title="Permissions Granted to List Owners"
                isOptional={true}
              />
            </Box>
            <Box mb={8}>
              <OwnersSection
                owners={owners}
                setOwners={setOwners}
                fetchRoleOptions={fetchRoleOptions}
                isDisabled={createAttempt.status === 'processing'}
                noAccess={userOptions.length === 0}
              />
            </Box>
            <Box>
              <MembersSection
                members={members}
                setMembers={setMembers}
                fetchRoleOptions={fetchRoleOptions}
                isDisabled={createAttempt.status === 'processing'}
                noAccess={userOptions.length === 0}
              />
            </Box>
            <Box mt={5} mb={8}>
              <ButtonPrimary
                onClick={() => handleOnCreate(validator)}
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
