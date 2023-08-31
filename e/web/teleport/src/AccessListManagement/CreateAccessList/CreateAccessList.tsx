import React, { useState, useEffect } from 'react';
import { useHistory } from 'react-router';
import { Link } from 'react-router-dom';
import {
  ButtonPrimary,
  ButtonSecondary,
  Alert,
  Box,
  Text,
  Flex,
  Indicator,
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

import { accessManagementService } from 'e-teleport/services/accessmanagement';
import cfg from 'e-teleport/config';

import { NoAccessState } from '../NoAccessState';
import {
  UserOption,
  auditFrequencyOpts,
  matchRoles,
  matchTraits,
} from '../Shared';
import { useFetchUserAndRoles } from '../useFetchUsersAndRoles';
import { TraitLookup, convertTraitLabelsToAllUserTraits } from '../Traits';

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

  const initAttemptObj = useAttempt('processing');
  const { attempt: initAttempt } = initAttemptObj;
  const { userOptions, roleOptions, fetchUsersAndRoles } =
    useFetchUserAndRoles(initAttemptObj);
  const { attempt: createAttempt, setAttempt: setCreateAttempt } =
    useAttempt('');

  const [spec, setSpec] = useState<Spec>({
    title: '',
    description: '',
    // Default to the max frequency
    auditFrequency: auditFrequencyOpts[auditFrequencyOpts.length - 1],
    auditStartDate: null,
  });

  const [grant, setGrant] = useState<Grant>({
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
    fetchUsersAndRoles();
  }, []);

  // Update owners.
  useEffect(() => {
    let eligibleOwners: UserOption[] = [];
    let selectedOwners: UserOption[] = [];
    const rolesRequiredToBeEligible = owners.selectedRolesRequired;

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

    setOwners({ ...owners, eligibleOwners, selectedOwners });
  }, [owners.selectedRolesRequired]);

  // Update members.
  useEffect(() => {
    let eligibleMembers: UserOption[] = [];
    let selectedMembers: UserOption[] = [];
    const rolesRequiredToBeEligible = members.selectedRolesRequired;

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

    setMembers({ ...members, eligibleMembers, selectedMembers });
  }, [members.selectedRolesRequired]);

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
        auditDuration: spec.auditFrequency.value,
        auditStartDate: spec.auditStartDate,
        // owners
        ownership_requires: {
          roles: owners.selectedRolesRequired.map(r => r.value),
          traits: convertTraitLabelsToAllUserTraits(owners.traitLabels),
        },
        owners: owners.selectedOwners.map(o => ({
          name: o.value.name,
        })),
        // members
        membership_requires: {
          roles: members.selectedRolesRequired.map(r => r.value),
          traits: convertTraitLabelsToAllUserTraits(owners.traitLabels),
        },
        members: members.selectedMembers.map(m => ({
          name: m.value.name,
          joined: new Date(),
          added_by: accessListCreater,
        })),
      })
      // After creating, go back to access list listing.
      .then(() => history.push(cfg.getAccessListManagementRoute()))
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
        {createAttempt.status === 'failed' && (
          <Alert children={createAttempt.statusText} />
        )}
        <Validation>
          {({ validator }) => (
            <Box width="540px">
              <Box mb={6}>
                <SpecSection
                  spec={spec}
                  setSpec={setSpec}
                  isDisabled={createAttempt.status === 'processing'}
                />
              </Box>
              <Box mb={8}>
                <GrantSection
                  grant={grant}
                  setGrant={setGrant}
                  roleOptions={roleOptions}
                  isDisabled={createAttempt.status === 'processing'}
                />
              </Box>
              <Box mb={8}>
                <OwnersSection
                  owners={owners}
                  setOwners={setOwners}
                  roleOptions={roleOptions}
                  isDisabled={createAttempt.status === 'processing'}
                  noAccess={userOptions.length === 0}
                />
              </Box>
              <Box>
                <MembersSection
                  members={members}
                  setMembers={setMembers}
                  roleOptions={roleOptions}
                  isDisabled={createAttempt.status === 'processing'}
                  noAccess={userOptions.length === 0}
                />
              </Box>
              <Box mt={5} mb={8}>
                <ButtonPrimary
                  width="170px"
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
  requiredTraitsToBeEligible: TraitLookup,
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
  selectedUsers: UserOption[];
}) {
  return selectedUsers.filter(selectedUser =>
    eligibleUsers.some(
      eligibleOwner => eligibleOwner.value.name === selectedUser.value.name
    )
  );
}
