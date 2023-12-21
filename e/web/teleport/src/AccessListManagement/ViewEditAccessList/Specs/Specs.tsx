import React, { useState } from 'react';
import { Flex, Text, Box } from 'design';
import { UserIdBadge, CircleCheck, NotificationsActive } from 'design/Icon';
import { Option } from 'shared/components/Select';

import { EditKind } from 'e-teleport/AccessListManagement/Shared/Shared';
import { getFormattedDate } from 'e-teleport/AccessListManagement/Shared/date';
import {
  getReviewDayOfMonthOption,
  getReviewFrequencyOption,
} from 'e-teleport/AccessListManagement/Shared/Audit';

import { AccessListModified } from '../ViewEditAccessList';
import { ButtonPencil, RoleAndTraitLabels } from '../Shared';

import { EditEligibilityOrGrantRoles } from './EditEligibilityOrGrants';
import { EditAudit } from './EditAudit';

type Props = {
  roleOptions: Option[];
  canEditSpecs: boolean;
  fetchAccessList(): Promise<void | boolean>;
  accessList: AccessListModified;
};

const genericNoAccessMsg = 'You do not have access to edit this access_list';

export function Specs({
  accessList,
  roleOptions,
  canEditSpecs,
  fetchAccessList,
}: Props) {
  const { membershipRequires, ownershipRequires, grants, audit } = accessList;
  const [editPermKind, setEditPermKind] = useState<EditKind>();
  const [showEditAudit, setShowEditAudit] = useState(false);

  const frequency = getReviewFrequencyOption(audit.recurrence.frequency).label;
  const dayOfMonth = getReviewDayOfMonthOption(
    audit.recurrence.dayOfMonth
  ).label;

  const editBtnTitle = canEditSpecs ? '' : genericNoAccessMsg;

  return (
    <>
      <Flex justifyContent="space-between">
        <Flex width="40%" mr={4} alignItems="flex-start">
          <CircleCheck />
          <Box>
            <Text ml={1} fontSize={4} mb={2}>
              Eligibility
            </Text>

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
        <Box width="40%">
          <Flex mb={2} alignItems="center" mt="-4px">
            <UserIdBadge mt="2px" />
            <Text ml={1} fontSize={4} mr={1}>
              Permissions Granted
            </Text>
            <ButtonPencil
              title={editBtnTitle}
              onClick={() => setEditPermKind('Grants')}
              disabled={!canEditSpecs}
            />
          </Flex>
          <RoleAndTraitLabels
            roles={grants.roles}
            traits={grants.traitList}
            required={true}
          />
        </Box>

        {/* Audit section */}
        <Box width="20%">
          <Flex mb={2} alignItems="center" mt="-4px">
            <NotificationsActive />
            <Text ml={1} fontSize={4} mr={1}>
              Audit
            </Text>
            <ButtonPencil
              title={editBtnTitle}
              onClick={() => setShowEditAudit(true)}
              disabled={!canEditSpecs}
            />
          </Flex>
          <Box mb={2}>
            <Text fontSize={1} mb={2}>
              Next Date: {getFormattedDate(audit.nextDate)}
            </Text>
            <Text fontSize={1}>
              Frequency: {frequency} {dayOfMonth}
            </Text>
          </Box>
        </Box>
      </Flex>
      {editPermKind && (
        <EditEligibilityOrGrantRoles
          onClose={() => setEditPermKind(null)}
          editKind={editPermKind}
          roleOptions={roleOptions}
          fetchAccessList={fetchAccessList}
          accessList={accessList}
        />
      )}
      {showEditAudit && (
        <EditAudit
          onClose={() => setShowEditAudit(false)}
          fetchAccessList={fetchAccessList}
          accessList={accessList}
        />
      )}
    </>
  );
}
