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
import userService from 'teleport/services/user';
import ResourceService from 'teleport/services/resources';
import useTeleport from 'teleport/useTeleport';

import { accessManagementService } from 'e-teleport/services/accessmanagement';
import cfg from 'e-teleport/config';

import { NoAccessState } from '../NoAccessState';
import { RoleOption, UserOption } from '../Shared';

import { auditFrequencyOpts, Spec, SpecSection } from './SpecSection';
import { Members, MembersSection } from './MemberSection';
import { Owners, OwnersSection } from './OwnerSection';

export function CreateAccessList() {
  const history = useHistory();
  const ctx = useTeleport();
  const perm = ctx.storeUser.getAccessListAccess();
  const canCreate = perm.create;
  const accessListCreater = ctx.storeUser.getUsername();

  const { attempt: initAttempt, run: initRun } = useAttempt('');
  const [fetchedUserOpts, setFetchedUserOpts] = useState<UserOption[]>([]);
  const [fetchedRoleOpts, setFetchedRoleOpts] = useState<RoleOption[]>([]);

  const { attempt: createAttempt, run: createRun } = useAttempt('');

  const [spec, setSpec] = useState<Spec>({
    title: '',
    description: '',
    // Default to the max frequency
    auditFrequency: auditFrequencyOpts[auditFrequencyOpts.length - 1],
    auditStartDate: null,
    rolesToGrant: [],
  });

  const [owners, setOwners] = useState<Owners>({
    selectedRolesRequired: [],
    eligibleOwners: [],
    selectedOwners: [],
  });

  const [members, setMembers] = useState<Members>({
    selectedRolesRequired: [],
    eligibleMembers: [],
    selectedMembers: [],
  });

  // Fetch initial users and roles.
  useEffect(() => {
    const resourceSvc = new ResourceService();
    initRun(() =>
      Promise.all([
        resourceSvc.fetchRoles().then(roles => {
          const madeRoleOpts = roles.map(role => ({
            value: role,
            label: role.name,
          }));
          setFetchedRoleOpts(madeRoleOpts);
        }),
        // Fetch all the existing users to filter
        // users who are eligible for being
        // owners or members depending on the
        // required roles defined.
        userService.fetchUsers().then(users => {
          const madeUserOpts = users.map(user => ({
            value: user,
            label: user.name,
          }));
          setFetchedUserOpts(madeUserOpts);
        }),
      ])
    );
  }, []);

  // Update owners.
  useEffect(() => {
    let eligibleOwners: UserOption[] = [];
    let selectedOwners: UserOption[] = [];
    const rolesRequiredToBeEligible = owners.selectedRolesRequired;

    if (rolesRequiredToBeEligible.length > 0) {
      eligibleOwners = eligibleUsers(
        rolesRequiredToBeEligible,
        fetchedUserOpts
      );
    }

    if (eligibleOwners.length > 0 && owners.selectedOwners.length > 0) {
      selectedOwners = eligibleUsersAmongSelectedUsers({
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

    if (rolesRequiredToBeEligible.length > 0) {
      eligibleMembers = eligibleUsers(
        rolesRequiredToBeEligible,
        fetchedUserOpts
      );
    }

    if (eligibleMembers.length > 0 && members.selectedMembers.length > 0) {
      selectedMembers = eligibleUsersAmongSelectedUsers({
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

    createRun(() =>
      accessManagementService
        .createAccessList({
          // specs
          title: spec.title,
          description: spec.description,
          grants: { roles: spec.rolesToGrant.map(r => r.value.name) },
          auditDuration: spec.auditFrequency.value,
          // owners
          ownership_requires: {
            roles: owners.selectedRolesRequired.map(r => r.value.name),
          },
          owners: owners.selectedOwners.map(o => ({
            name: o.value.name,
          })),
          // members
          membership_requires: {
            roles: members.selectedRolesRequired.map(r => r.value.name),
          },
          members: members.selectedMembers.map(m => ({
            name: m.value.name,
            joined: new Date(),
            added_by: accessListCreater,
          })),
        })
        // After creating, go back to access list listing.
        .then(() => history.push(cfg.getAccessListManagementRoute()))
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
                  fetchedRoleOpts={fetchedRoleOpts}
                  isDisabled={createAttempt.status === 'processing'}
                />
              </Box>
              <Box mb={6}>
                <OwnersSection
                  owners={owners}
                  setOwners={setOwners}
                  fetchedRoleOpts={fetchedRoleOpts}
                  isDisabled={createAttempt.status === 'processing'}
                />
              </Box>
              <Box>
                <MembersSection
                  members={members}
                  setMembers={setMembers}
                  fetchedRoleOpts={fetchedRoleOpts}
                  isDisabled={createAttempt.status === 'processing'}
                />
              </Box>
              <Box mt={5}>
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

// eligibleUsers returns users with roles
// that match with the required roles.
export function eligibleUsers(
  rolesRequiredToBeEligible: RoleOption[],
  users: UserOption[]
) {
  return users.filter(userOpt => {
    const currRolesAssigned = userOpt.value.roles;
    const isEligible = rolesRequiredToBeEligible.every(requiredRole =>
      currRolesAssigned.includes(requiredRole.value.name)
    );
    if (isEligible) {
      return userOpt;
    }
  });
}

// eligibleUsersAmongSelectedUsers checks if selected owners
// are still eligible and returns selected users who are found
// in the eligible list.
function eligibleUsersAmongSelectedUsers({
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
