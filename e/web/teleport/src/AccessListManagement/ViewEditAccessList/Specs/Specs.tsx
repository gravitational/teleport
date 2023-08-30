import React, { useState } from 'react';
import { Flex, Text, Box, ButtonIcon } from 'design';
import {
  UserIdBadge,
  Pencil,
  CircleCheck,
  NotificationsActive,
} from 'design/Icon';
import { Option } from 'shared/components/Select';

import {
  AccessListAudit,
  AccessListGrant,
  AccessListRequires,
} from 'e-teleport/services/accessmanagement';
import {
  TruncatingLabel,
  EditKind,
  calculateMonthsDaysFromDuration,
  getFormattedDate,
} from 'e-teleport/AccessListManagement/Shared';

import { EditAccess } from '../ViewEditAccessList';

import { EditEligibilityOrGrantRoles } from './EditEligibilityOrGrants';
import { EditAudit } from './EditAudit';

type Props = {
  membershipRequires: AccessListRequires;
  ownershipRequires: AccessListRequires;
  grants: AccessListGrant;
  audit: AccessListAudit;
  roleOptions: Option[];
  editAccess: EditAccess;
  fetchAccessList(): Promise<void | boolean>;
};

export function Specs({
  membershipRequires,
  ownershipRequires,
  grants,
  audit,
  roleOptions,
  editAccess,
  fetchAccessList,
}: Props) {
  const [editElibilityRoles, setEditEligibilityRoles] =
    useState<{ roles: string[]; kind: EditKind }>();
  const [showEditGrants, setShowEditGrants] = useState(false);
  const [showEditAudit, setShowEditAudit] = useState(false);

  function handleShowEditEligibility(kind: EditKind) {
    if (kind === 'Member') {
      setEditEligibilityRoles({ roles: membershipRequires.roles, kind });
      return;
    }
    setEditEligibilityRoles({ roles: ownershipRequires.roles, kind });
  }

  let frequencyTxt = '';
  const { months, days } = calculateMonthsDaysFromDuration(audit.frequency);
  if (months) {
    frequencyTxt = `${months} ${months > 1 ? 'months' : 'month'}`;
    if (days) {
      frequencyTxt = `${frequencyTxt} and ${days} ${days > 1 ? 'days' : 'day'}`;
    }
  } else if (days) {
    frequencyTxt = `${frequencyTxt} and ${days} ${days > 1 ? 'days' : 'day'}`;
  }

  return (
    <>
      <Flex justifyContent="space-between">
        <Flex width="33%" mr={4} alignItems="flex-start">
          <CircleCheck />
          <Box>
            <Text ml={1} fontSize={4} mb={2}>
              Eligibility: Required Roles
            </Text>

            {/* Owners section */}
            <Box mb={3} ml={1}>
              <Flex alignItems="center">
                <Text bold mr={1}>
                  List Owners
                </Text>
                <ButtonPencil
                  title={editAccess.owners.btnTitle}
                  onClick={() => handleShowEditEligibility('Owner')}
                  disabled={!editAccess.owners.hasAccess}
                />
              </Flex>
              <Flex alignItems="center">
                <Text fontSize={1} mr={1}>
                  Roles:
                </Text>
                {renderRoles(ownershipRequires.roles)}
              </Flex>
            </Box>

            {/* Members section */}
            <Box ml={1}>
              <Flex alignItems="center">
                <Text bold mr={1}>
                  Members
                </Text>
                <ButtonPencil
                  title={editAccess.members.btnTitle}
                  onClick={() => handleShowEditEligibility('Member')}
                  disabled={!editAccess.members.hasAccess}
                />
              </Flex>
              <Flex alignItems="center">
                <Text fontSize={1} mr={1}>
                  Roles:
                </Text>
                {renderRoles(membershipRequires.roles)}
              </Flex>
            </Box>
          </Box>
        </Flex>

        {/* Permissions granted section */}
        <Box width="33%">
          <Flex mb={2} alignItems="center" mt="-4px">
            <UserIdBadge mt="2px" />
            <Text ml={1} fontSize={4} mr={1}>
              Permissions Granted
            </Text>
            <ButtonPencil
              title={editAccess.grants.btnTitle}
              onClick={() => setShowEditGrants(true)}
              disabled={!editAccess.grants.hasAccess}
            />
          </Flex>
          <Flex alignItems="center">
            <Text mr={1} fontSize={1}>
              Roles:
            </Text>
            {renderRoles(grants.roles)}
          </Flex>
        </Box>

        {/* Audit section */}
        <Box width="33%">
          <Flex mb={2} alignItems="center" mt="-4px">
            <NotificationsActive />
            <Text ml={1} fontSize={4} mr={1}>
              Audit
            </Text>
            <ButtonPencil
              title={editAccess.audit.btnTitle}
              onClick={() => setShowEditAudit(true)}
              disabled={!editAccess.audit.hasAccess}
            />
          </Flex>
          <Box mb={2}>
            <Text fontSize={1} mb={2}>
              Next Date: {getFormattedDate(audit.nextDate)}
            </Text>
            <Text fontSize={1}>Frequency: every {frequencyTxt}</Text>
          </Box>
        </Box>
      </Flex>
      {editElibilityRoles && (
        <EditEligibilityOrGrantRoles
          onClose={() => setEditEligibilityRoles(null)}
          existingRoles={editElibilityRoles.roles}
          editKind={editElibilityRoles.kind}
          roleOptions={roleOptions}
          fetchAccessList={fetchAccessList}
        />
      )}
      {showEditGrants && (
        <EditEligibilityOrGrantRoles
          onClose={() => setShowEditGrants(false)}
          existingRoles={grants.roles}
          roleOptions={roleOptions}
          fetchAccessList={fetchAccessList}
          editKind="Grants"
        />
      )}
      {showEditAudit && (
        <EditAudit
          onClose={() => setShowEditAudit(false)}
          audit={audit}
          fetchAccessList={fetchAccessList}
        />
      )}
    </>
  );
}

function ButtonPencil({
  onClick,
  disabled,
  title,
}: {
  onClick(): void;
  disabled: boolean;
  title: string;
}) {
  return (
    <ButtonIcon
      alignItems="center"
      onClick={onClick}
      disabled={disabled}
      title={title}
    >
      <Pencil size={16} />
    </ButtonIcon>
  );
}

const renderRoles = (labels: string[] = []) => {
  const $labels = labels.map((label, index) => (
    <TruncatingLabel
      mr={index === labels.length - 1 ? 0 : 1}
      key={`${label}${index}`}
      kind="secondary"
      title={label}
    >
      {label}
    </TruncatingLabel>
  ));

  return <Flex flexWrap="wrap">{$labels}</Flex>;
};
