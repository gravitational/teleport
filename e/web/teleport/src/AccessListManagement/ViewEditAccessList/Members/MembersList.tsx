import React, { useState } from 'react';
import { Flex, Text, Box, ButtonText, ButtonSecondary } from 'design';
import Table, { StyledPanel } from 'design/DataTable';
import { UsersTriple, Add, ArrowRight } from 'design/Icon';
import { HoverTooltip } from 'shared/components/ToolTip';
import { PagedTableProps } from 'design/DataTable/types';
import InputSearch from 'design/DataTable/InputSearch';
import { StyledTable } from 'design/DataTable/StyledTable';
import { ClientSidePager } from 'design/DataTable/Pager';
import { getPagerPosition } from 'design/DataTable/Table';
import { useClientSidePager } from 'design/DataTable/Pager/ClientSidePager/useClientSidePager';

import { H2 } from 'design';

import {
  AccessList,
  AccessListMember,
} from 'e-teleport/services/accessmanagement';
import { getFormattedDate } from 'e-teleport/AccessListManagement/Shared/date';

import { UserOption } from '../../Shared/Shared';

import { CustomCell, UserRevokeButtonCell } from '../Shared';
import { DeleteUserConfirmDialog } from '../DeleteUserConfirmDialog';
import { AccessListModified } from '../ViewEditAccessList';

import { EnrollNewMembers } from './EnrollNewMembers';

const genericNoAccessMsg = 'You do not have access to edit members';

export function MembersList({
  accessList,
  userOptions,
  canEditMembers,
  updateAccessList,
}: {
  userOptions: UserOption[];
  canEditMembers: boolean;
  updateAccessList(accessList: AccessList): void;
  accessList: AccessListModified;
}) {
  const { members } = accessList;
  const [showEnrollNewMembers, setShowEnrollNewMembers] = useState(false);
  const [deleteMember, setDeleteMember] = useState<AccessListMember>();
  return (
    <>
      <Box>
        <Flex justifyContent="space-between" mb={2}>
          <Flex alignItems="center">
            <UsersTriple />
            <H2 ml={1} mr={2}>
              Members
            </H2>
          </Flex>
          <ButtonText
            title={canEditMembers ? '' : genericNoAccessMsg}
            disabled={!canEditMembers}
            onClick={() => setShowEnrollNewMembers(true)}
            mr={0}
          >
            <Add size={16} mr={2} />
            Enroll New Members
          </ButtonText>
        </Flex>
      </Box>
      <AccessListMemberTable
        members={members}
        canEditMembers={canEditMembers}
        onDeleteMember={setDeleteMember}
      />
      {showEnrollNewMembers && (
        <EnrollNewMembers
          onClose={() => setShowEnrollNewMembers(false)}
          accessList={accessList}
          userOptions={userOptions}
          updateAccessList={updateAccessList}
        />
      )}
      {deleteMember && (
        <DeleteUserConfirmDialog
          onClose={() => setDeleteMember(null)}
          kind="Member"
          username={deleteMember.name}
          accessList={accessList}
          updateAccessList={updateAccessList}
        />
      )}
    </>
  );
}

export const AccessListMemberTable = ({
  members,
  canEditMembers,
  onDeleteMember = null,
  hideIneligibleReason = false,
  hideReasonCol = false,
  isReviewing = false,
}: {
  members: AccessListMember[];
  canEditMembers: boolean;
  onDeleteMember?(m: AccessListMember): void;
  hideIneligibleReason?: boolean;
  hideReasonCol?: boolean;
  isReviewing?: boolean;
}) => {
  return (
    <Table
      data={members}
      columns={[
        {
          key: 'name',
          headerText: 'Name',
          isSortable: true,
          render: ({ name, ineligibleReason }) => (
            <CustomCell disabled={!hideIneligibleReason && !!ineligibleReason}>
              {name}
            </CustomCell>
          ),
        },
        {
          key: 'addedBy',
          headerText: 'Added By',
          isSortable: true,
          render: ({ addedBy, ineligibleReason }) => (
            <CustomCell disabled={!hideIneligibleReason && !!ineligibleReason}>
              {addedBy}
            </CustomCell>
          ),
        },
        !hideReasonCol && {
          key: 'reason',
          headerText: 'Reason',
          isSortable: true,
          render: ({ reason, ineligibleReason }) => (
            <CustomCell disabled={!hideIneligibleReason && !!ineligibleReason}>
              {reason}
            </CustomCell>
          ),
        },
        {
          key: 'joined',
          headerText: 'Date Added',
          isSortable: true,
          onSort: sortCustomDate,
          render: ({ joined, ineligibleReason }) => (
            <CustomCell disabled={!hideIneligibleReason && !!ineligibleReason}>
              {getFormattedDate(joined)}
            </CustomCell>
          ),
        },
        {
          key: 'expires',
          headerText: 'Expires',
          isSortable: true,
          render: ({ expires, ineligibleReason }) => (
            <CustomCell disabled={!hideIneligibleReason && !!ineligibleReason}>
              {getFormattedDate(expires)}
            </CustomCell>
          ),
        },
        {
          altKey: 'options-btn',
          isNonRender: !onDeleteMember,
          render: member => (
            <UserRevokeButtonCell
              disabled={!canEditMembers}
              btnTitle={canEditMembers ? '' : genericNoAccessMsg}
              onClick={() => onDeleteMember(member)}
              ineligibleReason={member.ineligibleReason}
              hideIneligibleReason={hideIneligibleReason}
              isReviewing={isReviewing}
            />
          ),
        },
      ]}
      emptyText="No Users Found"
      isSearchable
      pagination={{
        pageSize: 10,
        pagerPosition: isReviewing ? 'both' : 'top',
        CustomTable: isReviewing ? CustomTable : undefined,
      }}
    />
  );
};

function sortCustomDate(a: Date, b: Date) {
  const aStr = getFormattedDate(a);
  const bStr = getFormattedDate(b);

  if (aStr < bStr) {
    return -1;
  }
  if (aStr > bStr) {
    return 1;
  }

  return 0;
}

function CustomTable<T>({
  nextPage,
  prevPage,
  renderHeaders,
  renderBody,
  data,
  pagination,
  searchValue,
  setSearchValue,
  fetching,
  className,
  style,
}: PagedTableProps<T>) {
  const { pagerPosition, paginatedData, currentPage } = pagination;
  const { showBothPager, showBottomPager, showTopPager } = getPagerPosition(
    pagerPosition,
    paginatedData[currentPage].length
  );

  const { isNextDisabled } = useClientSidePager({
    data,
    paginatedData,
    currentPage,
    pageSize: pagination.pageSize,
    nextPage,
    prevPage,
  });

  return (
    <>
      <StyledPanel>
        <InputSearch
          searchValue={searchValue}
          setSearchValue={setSearchValue}
        />
        {(showTopPager || showBothPager) && (
          <ClientSidePager
            nextPage={nextPage}
            prevPage={prevPage}
            data={data}
            {...fetching}
            {...pagination}
          />
        )}
      </StyledPanel>
      <StyledTable className={className} style={style}>
        {renderHeaders()}
        {renderBody(paginatedData[currentPage])}
      </StyledTable>
      <Flex gap={2} justifyContent="space-between" alignItems="center" mt={3}>
        <HoverTooltip
          tipContent={
            !isNextDisabled ? 'More members next page' : 'End of page'
          }
        >
          <ButtonSecondary
            textTransform="none"
            width="150px"
            disabled={isNextDisabled}
            onClick={nextPage}
          >
            <Flex alignItems="center" gap={2}>
              <Text>View More</Text> <ArrowRight size={16} />
            </Flex>
          </ButtonSecondary>
        </HoverTooltip>
        {(showBottomPager || showBothPager) && (
          <ClientSidePager
            nextPage={nextPage}
            prevPage={prevPage}
            data={data}
            {...pagination}
          />
        )}
      </Flex>
    </>
  );
}
