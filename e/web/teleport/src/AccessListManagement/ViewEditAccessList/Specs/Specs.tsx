import React, { useState } from 'react';
import { Flex, Text, Box, H2, ButtonText } from 'design';
import { UserIdBadge, CircleCheck, NotificationsActive } from 'design/Icon';
import { Option } from 'shared/components/Select';

import { EditKind } from 'e-teleport/AccessListManagement/Shared/Shared';
import { getFormattedDate } from 'e-teleport/AccessListManagement/Shared/date';
import {
  getReviewDayOfMonthOption,
  getReviewFrequencyOption,
} from 'e-teleport/AccessListManagement/Shared/Audit';
import { AccessList } from 'e-teleport/services/accessmanagement';

import { convertToTraitConvenience } from 'e-teleport/AccessListManagement/Traits';

import {
  AccessListModified,
  ButtonPencil,
  RoleAndTraitLabels,
  MAX_DISPLAYED_INHERITED_ROLES_TRAITS,
} from '../Shared';

import { EditEligibilityOrGrantRoles } from './EditEligibilityOrGrants';
import { EditAudit } from './EditAudit';
import ShowMoreGrantsDialog from './ShowMoreGrantsDialog';

type Props = {
  fetchRoleOptions: (input: string) => Promise<Option[]>;
  canEditSpecs: boolean;
  updateAccessList(accessList: AccessList): void;
  accessList: AccessListModified;
};

const genericNoAccessMsg = 'You do not have access to edit this access_list';

export function Specs({
  accessList,
  fetchRoleOptions,
  canEditSpecs,
  updateAccessList,
}: Props) {
  const {
    membershipRequires,
    ownershipRequires,
    grants,
    audit,
    ownerGrants,
    inheritedMemberGrants,
  } = accessList;
  const [editPermKind, setEditPermKind] = useState<EditKind>();
  const [showEditAudit, setShowEditAudit] = useState(false);
  const [showMoreGrantsDialog, setShowMoreGrantsDialog] = useState(false);

  const frequency = getReviewFrequencyOption(audit.recurrence.frequency).label;
  const dayOfMonth = getReviewDayOfMonthOption(
    audit.recurrence.dayOfMonth
  ).label;

  const editBtnTitle = canEditSpecs ? '' : genericNoAccessMsg;

  const inheritedMemberRoles = inheritedMemberGrants.roles;
  const inheritedMemberTraits = convertToTraitConvenience(
    inheritedMemberGrants.traits
  ).traitList;

  const truncateInheritedRolesTraits =
    inheritedMemberRoles.length > MAX_DISPLAYED_INHERITED_ROLES_TRAITS ||
    inheritedMemberTraits.length > MAX_DISPLAYED_INHERITED_ROLES_TRAITS;

  return (
    <>
      <Flex justifyContent="space-between">
        <Flex width="40%" mr={4} alignItems="flex-start">
          <CircleCheck />
          <Box>
            <H2 ml={1} mb={2}>
              Eligibility
            </H2>

            {/* Owners section */}
            <Box mb={3} ml={1}>
              <Flex alignItems="center">
                <Text bold mr={1}>
                  List Owners
                </Text>
                <ButtonPencil
                  title={editBtnTitle}
                  onClick={() => setEditPermKind('Owner')}
                  disabled={!canEditSpecs}
                />
              </Flex>
              <RoleAndTraitLabels
                roles={ownershipRequires.roles}
                traits={ownershipRequires.traitList}
              />
            </Box>

            {/* Members section */}
            <Box ml={1}>
              <Flex alignItems="center">
                <Text bold mr={1}>
                  Members
                </Text>
                <ButtonPencil
                  title={editBtnTitle}
                  onClick={() => setEditPermKind('Member')}
                  disabled={!canEditSpecs}
                />
              </Flex>
              <RoleAndTraitLabels
                roles={membershipRequires.roles}
                traits={membershipRequires.traitList}
              />
            </Box>
          </Box>
        </Flex>

        {/* Permissions granted section */}
        <Flex width="40%" mr={4} alignItems="flex-start">
          <UserIdBadge mt="2px" />
          <Box>
            <H2 ml={1} mb={2}>
              Permissions Granted
            </H2>

            {/* Owners grant section */}
            <Box mb={3} ml={1}>
              <Flex alignItems="center" gap={1}>
                <Text bold>List Owners</Text>
                <ButtonPencil
                  title={editBtnTitle}
                  onClick={() => setEditPermKind('OwnerGrants')}
                  disabled={!canEditSpecs}
                />
              </Flex>
              <RoleAndTraitLabels
                roles={ownerGrants.roles}
                traits={ownerGrants.traitList}
                required
              />
            </Box>

            {/* Members grant section */}
            <Box ml={1}>
              <Flex alignItems="center" gap={1}>
                <Text bold>Members</Text>
                <ButtonPencil
                  title={editBtnTitle}
                  onClick={() => setEditPermKind('Grants')}
                  disabled={!canEditSpecs}
                />
              </Flex>
              <RoleAndTraitLabels
                roles={grants.roles}
                traits={grants.traitList}
                required
              />
            </Box>
          </Box>
        </Flex>
        {/* Inherited  permissions section */}
        {(inheritedMemberRoles?.length > 0 ||
          inheritedMemberTraits?.length > 0) && (
          <Flex width="40%" mr={4} alignItems="flex-start">
            <UserIdBadge mt="2px" />
            <Box>
              <H2 ml={1} mb={2}>
                Inherited Permissions
              </H2>
              {/* Members grant section */}
              <Box ml={1}>
                <Flex gap={1} flexDirection="column">
                  <Text bold>Members</Text>
                  <RoleAndTraitLabels
                    roles={inheritedMemberRoles}
                    traits={inheritedMemberTraits}
                    required
                    truncate
                  />
                  {truncateInheritedRolesTraits && (
                    <ButtonText
                      size="small"
                      onClick={() => setShowMoreGrantsDialog(true)}
                      textTransform="none"
                      mt={1}
                    >
                      Show More
                    </ButtonText>
                  )}
                </Flex>
              </Box>
            </Box>
          </Flex>
        )}

        {/* Audit section */}
        <Box width="20%">
          <Flex mb={2} alignItems="center" mt="-4px">
            <NotificationsActive />
            <H2 ml={1} mr={1}>
              Audit
            </H2>
            <ButtonPencil
              title={editBtnTitle}
              onClick={() => setShowEditAudit(true)}
              disabled={!canEditSpecs}
            />
          </Flex>
          <Box mb={2}>
            <Text typography="body3" mb={2}>
              Next Date: {getFormattedDate(audit.nextDate)}
            </Text>
            <Text typography="body3">
              Frequency: {frequency} {dayOfMonth}
            </Text>
          </Box>
        </Box>
      </Flex>
      {editPermKind && (
        <EditEligibilityOrGrantRoles
          onClose={() => setEditPermKind(null)}
          editKind={editPermKind}
          fetchRoleOptions={fetchRoleOptions}
          updateAccessList={updateAccessList}
          accessList={accessList}
        />
      )}
      {showEditAudit && (
        <EditAudit
          onClose={() => setShowEditAudit(false)}
          updateAccessList={updateAccessList}
          accessList={accessList}
        />
      )}
      {showMoreGrantsDialog && (
        <ShowMoreGrantsDialog
          roles={inheritedMemberRoles}
          traits={inheritedMemberTraits}
          onClose={() => setShowMoreGrantsDialog(false)}
        />
      )}
    </>
  );
}
