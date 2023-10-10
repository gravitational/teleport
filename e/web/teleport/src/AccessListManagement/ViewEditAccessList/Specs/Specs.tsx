import React, { useState } from 'react';
import styled from 'styled-components';
import { Flex, Text, Box, ButtonIcon } from 'design';
import {
  UserIdBadge,
  Pencil,
  CircleCheck,
  NotificationsActive,
} from 'design/Icon';
import { Option } from 'shared/components/Select';

import {
  TruncatingLabel,
  EditKind,
} from 'e-teleport/AccessListManagement/Shared/Shared';
import { getFormattedDate } from 'e-teleport/AccessListManagement/Shared/date';
import {
  getReviewDayOfMonthOption,
  getReviewFrequencyOption,
} from 'e-teleport/AccessListManagement/Shared/Audit';

import { AccessListModified, EditAccess } from '../ViewEditAccessList';

import { EditEligibilityOrGrantRoles } from './EditEligibilityOrGrants';
import { EditAudit } from './EditAudit';

type Props = {
  roleOptions: Option[];
  editAccess: EditAccess;
  fetchAccessList(): Promise<void | boolean>;
  accessList: AccessListModified;
};

export function Specs({
  accessList,
  roleOptions,
  editAccess,
  fetchAccessList,
}: Props) {
  const { membershipRequires, ownershipRequires, grants, audit } = accessList;
  const [editPermKind, setEditPermKind] = useState<EditKind>();
  const [showEditAudit, setShowEditAudit] = useState(false);

  const frequency = getReviewFrequencyOption(audit.recurrence.frequency).label;
  const dayOfMonth = getReviewDayOfMonthOption(
    audit.recurrence.dayOfMonth
  ).label;

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
                  title={editAccess.owners.btnTitle}
                  onClick={() => setEditPermKind('Owner')}
                  disabled={!editAccess.owners.hasAccess}
                />
              </Flex>
              {ownershipRequires.roles.length > 0 && (
                <Flex alignItems="center">
                  <TextNoEllipsis mr={1}>Roles:</TextNoEllipsis>
                  {renderRoles(ownershipRequires.roles)}
                </Flex>
              )}
              {ownershipRequires.traitList.length > 0 && (
                <Flex alignItems="center">
                  <TextNoEllipsis mr={1}>Traits:</TextNoEllipsis>
                  {renderRoles(ownershipRequires.traitList)}
                </Flex>
              )}
            </Box>

            {/* Members section */}
            <Box ml={1}>
              <Flex alignItems="center">
                <Text bold mr={1}>
                  Members
                </Text>
                <ButtonPencil
                  title={editAccess.members.btnTitle}
                  onClick={() => setEditPermKind('Member')}
                  disabled={!editAccess.members.hasAccess}
                />
              </Flex>
              {membershipRequires.roles.length > 0 && (
                <Flex alignItems="center">
                  <TextNoEllipsis mr={1}>Roles:</TextNoEllipsis>
                  {renderRoles(membershipRequires.roles)}
                </Flex>
              )}
              {membershipRequires.traitList.length > 0 && (
                <Flex alignItems="center">
                  <TextNoEllipsis mr={1}>Traits:</TextNoEllipsis>
                  {renderRoles(membershipRequires.traitList)}
                </Flex>
              )}
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
              title={editAccess.grants.btnTitle}
              onClick={() => setEditPermKind('Grants')}
              disabled={!editAccess.grants.hasAccess}
            />
          </Flex>
          {grants.roles.length > 0 && (
            <Flex alignItems="center">
              <TextNoEllipsis mr={1}>Roles:</TextNoEllipsis>
              {renderRoles(grants.roles)}
            </Flex>
          )}
          {grants.traitList.length > 0 && (
            <Flex alignItems="center">
              <TextNoEllipsis mr={1}>Traits:</TextNoEllipsis>
              {renderRoles(grants.traitList)}
            </Flex>
          )}
        </Box>

        {/* Audit section */}
        <Box width="20%">
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

const TextNoEllipsis = styled(Box)`
  font-size: ${p => p.theme.fontSizes[1]}px;
`;
