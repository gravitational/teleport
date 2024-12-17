import { useState } from 'react';
import { Flex, Text, Box, ButtonText, ButtonSecondary, H2 } from 'design';
import Table, { StyledPanel } from 'design/DataTable';
import { UsersTriple, Add, ArrowRight } from 'design/Icon';
import { HoverTooltip, ToolTipInfo } from 'shared/components/ToolTip';
import InputSearch from 'design/DataTable/InputSearch';
import { StyledTable } from 'design/DataTable/StyledTable';
import { ClientSidePager } from 'design/DataTable/Pager';
import { getPagerPosition } from 'design/DataTable/Table';
import { useClientSidePager } from 'design/DataTable/Pager/ClientSidePager/useClientSidePager';

import {
  AccessListMember,
  AccessListMemberKind,
} from 'e-teleport/services/accessmanagement';
import { getFormattedDate } from 'e-teleport/AccessListManagement/Shared/date';
import { useOnClickNestedList } from 'e-teleport/AccessListManagement/Shared/nav';

import { NestedListLink } from '../../Shared/Shared';

import { CustomCell, UserRevokeButtonCell } from '../Shared';
import { DeleteUserConfirmDialog } from '../DeleteUserConfirmDialog';

import { EnrollNewMembers } from './EnrollNewMembers';

import type { PagedTableProps } from 'design/DataTable/types';
import type { AccessListModified } from '../Shared';
import type { UserOption } from '../../Shared/Shared';
import type { AccessList } from 'e-teleport/services/accessmanagement';
import type { AccessListWithModifiedGrants } from 'e-teleport/AccessListManagement/AccessLists/AccessLists';

const genericNoAccessMsg = 'You do not have access to edit members';

export function MembersList({
  accessList,
  userOptions,
  canEditMembers,
  updateAccessList,
  accessLists,
}: {
  userOptions: UserOption[];
  canEditMembers: boolean;
  updateAccessList(accessList: AccessList, members?: AccessListMember[]): void;
  accessList: AccessListModified;
  accessLists: AccessListWithModifiedGrants[];
}) {
  const { members } = accessList;
  const [showEnrollNewMembers, setShowEnrollNewMembers] = useState(false);
  const [deleteMember, setDeleteMember] =
    useState<(typeof accessList)['members'][number]>();

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
            Enroll New Members or Access Lists
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
          accessLists={accessLists}
        />
      )}
      {deleteMember && (
        <DeleteUserConfirmDialog
          onClose={() => setDeleteMember(null)}
          kind="Member"
          username={deleteMember.name}
          displayName={
            deleteMember.membershipKind === AccessListMemberKind.List
              ? deleteMember.title
              : undefined
          }
          accessList={accessList}
          updateAccessList={l =>
            updateAccessList(
              l,
              members.filter(m => m !== deleteMember)
            )
          }
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
  members: AccessListModified['members'];
  canEditMembers: boolean;
  onDeleteMember?(m: AccessListModified['members'][number]): void;
  hideIneligibleReason?: boolean;
  hideReasonCol?: boolean;
  isReviewing?: boolean;
}) => {
  const onClickNestedList = useOnClickNestedList();

  return (
    <Table
      data={members}
      columns={[
        {
          key: 'membershipKind',
          isSortable: true,
          headerText: 'Type',
          onSort: (a, b) => {
            if (a?.membershipKind === b?.membershipKind) {
              return 0;
            }
            return a?.membershipKind === AccessListMemberKind.List ? -1 : 1;
          },
          render: ({ membershipKind, ineligibleReason }) => (
            <CustomCell disabled={!hideIneligibleReason && !!ineligibleReason}>
              {membershipKind === AccessListMemberKind.List
                ? 'Access List'
                : 'User'}
            </CustomCell>
          ),
        },
        {
          key: 'name',
          headerText: 'Name',
          isSortable: true,
          render: ({ name, ineligibleReason, title, ...rest }) => {
            if (rest.membershipKind === AccessListMemberKind.List) {
              return (
                <CustomCell
                  disabled={false}
                  title={
                    hideIneligibleReason || rest.accessListExists
                      ? `View list '${title}'`
                      : ''
                  }
                >
                  <NestedListLink
                    title={
                      hideIneligibleReason || rest.accessListExists
                        ? `View list '${title}'`
                        : ''
                    }
                    onClick={() => onClickNestedList(name)}
                    disabled={!hideIneligibleReason && !rest.accessListExists}
                  >
                    {title}
                    {!hideIneligibleReason && !rest.accessListExists && (
                      <ToolTipInfo
                        kind="warning"
                        children={`Insufficient permissions to view list '${title}'`}
                        css={`
                          margin-left: 5px;
                        `}
                      />
                    )}
                  </NestedListLink>
                </CustomCell>
              );
            }

            return (
              <CustomCell
                disabled={!hideIneligibleReason && !!ineligibleReason}
              >
                {name}
              </CustomCell>
            );
          },
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
          onSort: (a, b) => {
            const aStr = getFormattedDate(a.joined);
            const bStr = getFormattedDate(b.joined);

            if (aStr < bStr) {
              return -1;
            }
            if (aStr > bStr) {
              return 1;
            }

            return 0;
          },
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
          render: ({ expires, ineligibleReason, membershipKind }) => (
            <CustomCell disabled={!hideIneligibleReason && !!ineligibleReason}>
              {membershipKind === AccessListMemberKind.List
                ? ''
                : getFormattedDate(expires)}
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
      initialSort={{ key: 'name', dir: 'ASC' }}
    />
  );
};

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
            width="160px"
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
