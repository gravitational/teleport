import React, { useEffect, useState } from 'react';
import { useHistory } from 'react-router';
import { Link } from 'react-router-dom';
import {
  Alert,
  Box,
  ButtonPrimary,
  ButtonSecondary,
  Flex,
  Indicator,
  Text,
} from 'design';
import { ArrowBack } from 'design/Icon';
import useAttempt from 'shared/hooks/useAttemptNext';
import {
  FeatureBox,
  FeatureHeader,
  FeatureHeaderTitle,
} from 'teleport/components/Layout';
import Validation, { Validator } from 'shared/components/Validation';
import useTeleport from 'teleport/useTeleport';
import { Option } from 'shared/components/Select';
import { AllUserTraits } from 'teleport/services/user';

import {
  accessManagementService,
  convertReviewFrequencyIntoBackendParsableValue,
  ReviewDayOfMonth,
  ReviewFrequency,
} from 'e-teleport/services/accessmanagement';
import cfg from 'e-teleport/config';

import { NoAccessState } from '../NoAccessState';
import {
  HybridUserOption,
  matchRoles,
  matchTraits,
  UserOption,
} from '../Shared/Shared';
import { reviewDayOfMonthOpts, reviewFrequencyOpts } from '../Shared/Audit';
import { useFetchUserAndRoles } from '../useFetchUsersAndRoles';
import { convertTraitLabelsToAllUserTraits } from '../Traits';
import {
  FeatureLimitReached,
  featureLimitReachedBlurCss,
} from '../Shared/FeatureLimitReached';

import { Spec, SpecSection } from './SpecSection';
import { Members, MembersSection } from './MemberSection';
import { Owners, OwnersSection } from './OwnerSection';
import { Grant, GrantSection } from './GrantSection';

export function CreateAccessList() {
  const history = useHistory();
  const ctx = useTeleport();
  const perm = ctx.storeUser.getAccessListAccess();
  const canCreate = perm.create;
  const accessListCreater = ctx.storeUser.getUsername();

  const [featureLimitReached, setFeatureLimitReached] = useState(false);

  const initAttemptObj = useAttempt('processing');
  const { attempt: initAttempt, setAttempt: setInitAttempt } = initAttemptObj;
  const { userOptions, fetchRoleOptions, fetchUsersAndRoles } =
    useFetchUserAndRoles(initAttemptObj);
  const { attempt: createAttempt, setAttempt: setCreateAttempt } =
    useAttempt('');

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

  useEffect(() => {
    if (
      cfg.oss.entitlements.accessLists.enabled &&
      cfg.oss.entitlements.accessLists.limit === 0
    ) {
      fetchUsersAndRoles();
      return;
    }

    // Check if user has reached limit.
    accessManagementService
      .fetchAccessLists()
      .then(resp => {
        if (
          resp.length &&
          resp.length >= cfg.oss.entitlements.accessLists.limit
        ) {
          setFeatureLimitReached(true);
          setInitAttempt({ status: 'success' });
        } else {
          fetchUsersAndRoles();
        }
      })
      .catch((err: Error) => {
        setInitAttempt({ status: 'failed', statusText: err.message });
      });
  }, []);

  // Update owners.
  useEffect(() => {
    let eligibleOwners: UserOption[] = userOptions;
    let selectedOwners: HybridUserOption[] = [];
    const rolesRequiredToBeEligible = owners.selectedRolesRequired;

    // Only filter for eligible owners if required roles or traits
    // are defined. Otherwise all users are eligible.
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

    setOwners({ ...owners, eligibleOwners, selectedOwners });
  }, [userOptions, owners.selectedRolesRequired, owners.traitLabels]);

  // Update members.
  useEffect(() => {
    let eligibleMembers: UserOption[] = userOptions;
    let selectedMembers: HybridUserOption[] = [];
    const rolesRequiredToBeEligible = members.selectedRolesRequired;

    // Only filter for eligible members if required roles or traits
    // are defined. Otherwise all users are eligible.
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

    setMembers({ ...members, eligibleMembers, selectedMembers });
  }, [userOptions, members.selectedRolesRequired, members.traitLabels]);

  function handleOnCreate(validator: Validator) {
    if (!validator.validate()) {
      return;
    }

    // We don't need to setAttempt to "success"
    // since we are unmounting right after updating.
    setCreateAttempt({ status: 'processing' });
    accessManagementService
      .createAccessList({
        // specs
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
        owners: owners.selectedOwners.map(o => ({
          name: typeof o.value === 'string' ? o.value : o.value.name,
        })),
        // members
        membership_requires: {
          roles: members.selectedRolesRequired.map(r => r.value),
          traits: convertTraitLabelsToAllUserTraits(members.traitLabels),
        },
        members: members.selectedMembers.map(m => ({
          name: typeof m.value === 'string' ? m.value : m.value.name,
          joined: new Date(),
          added_by: accessListCreater,
        })),
      })
      // After creating, go back to access list listing.
      // Because of backend caching, we send the created list
      // as router state to be used to update the listing.
      .then(createdList => {
        history.push(cfg.getAccessListManagementRoute(), { createdList });
      })
      .catch((e: Error) =>
        setCreateAttempt({ status: 'failed', statusText: e.message })
      );
  }

  let MainContent: React.ReactElement;
  if (!canCreate) {
    MainContent = <NoAccessState action="create" />;
  } else if (initAttempt.status === 'processing') {
    MainContent = (
      <Box textAlign="center" m={10}>
        <Indicator />
      </Box>
    );
  } else if (initAttempt.status === 'failed') {
    MainContent = <Alert children={initAttempt.statusText} />;
  } else if (initAttempt.status === 'success') {
    MainContent = (
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
  }

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
            <Text fontSize="22px">Create a New Access List</Text>
          </Flex>
        </FeatureHeaderTitle>
      </FeatureHeader>
      {MainContent}
    </FeatureBox>
  );
}

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
