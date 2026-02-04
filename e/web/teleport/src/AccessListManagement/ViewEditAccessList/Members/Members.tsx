import { useState } from 'react';

import { Box, ButtonSecondary, ButtonText, Flex, Text } from 'design';
import Table, { StyledPanel } from 'design/DataTable';
import InputSearch from 'design/DataTable/InputSearch';
import { ClientSidePager } from 'design/DataTable/Pager';
import { useClientSidePager } from 'design/DataTable/Pager/ClientSidePager/useClientSidePager';
import { StyledTable } from 'design/DataTable/StyledTable';
import { getPagerPosition } from 'design/DataTable/Table';
import type { PagedTableProps } from 'design/DataTable/types';
import { Add, ArrowRight } from 'design/Icon';
import { HoverTooltip, IconTooltip } from 'design/Tooltip';

import { getFormattedDate } from 'e-teleport/AccessListManagement/Shared/date';
import { useOnClickNestedList } from 'e-teleport/AccessListManagement/Shared/nav';
import { convertToTraitConvenience } from 'e-teleport/AccessListManagement/Traits';
import {
  AccessListMember,
  AccessListMemberKind,
  isScim,
  type AccessList,
} from 'e-teleport/services/accessmanagement';

import { EditKind, NestedListLink } from '../../Shared/Shared';
import { Action, getActionForbiddenInfo, isActionForbidden } from '../access';
import { DeleteUserConfirmDialog } from '../DeleteUserConfirmDialog';
import {
  CustomCell,
  Perms,
  RoleAndTraitLabels,
  UserRevokeButtonCell,
  type AccessListModified,
} from '../Shared';
import { EditEligibilityOrGrantRoles } from '../Specs/EditEligibilityOrGrants';
import { EnrollNewMembers } from './EnrollNewMembers';

interface MembersProps {
  updateAccessList(accessList: AccessList, members?: AccessListMember[]): void;
  accessList: AccessListModified;
  isReadOnlyOktaList?: boolean;
  perms: Perms;
  switchToAccessDefinitionTab(): void;
}

export function Members(props: MembersProps) {
  const { accessList, updateAccessList, isReadOnlyOktaList, perms } = props;
  const { members, membershipRequires, inheritedMemberGrants, grants } =
    accessList;
  const [showEnrollNewMembers, setShowEnrollNewMembers] = useState(false);
  const [deleteMember, setDeleteMember] =
    useState<(typeof accessList)['members'][number]>();

  const [editPermKind, setEditPermKind] = useState<EditKind>();

  const inheritedMemberRoles = inheritedMemberGrants.roles;
  const inheritedMemberTraits = convertToTraitConvenience(
    inheritedMemberGrants.traits
  ).traitList;

  const canReadMembers = perms.isOwner || perms.adminWhoCanRead;

  return (
    <Box data-testid="members-content">
      <Flex justifyContent="space-between" gap={1} alignItems="flex-start">
        <Flex flexDirection="column" gap={3}>
          <Flex mb={4} flexDirection="column" gap={1}>
            <RoleAndTraitLabels
              roles={membershipRequires.roles}
              traits={membershipRequires.traitList}
              accessKind="requirements"
              toolTipContent={getActionForbiddenInfo({
                action: Action.EditMembersEligibility,
                ...props,
              })}
              editDisabled={isActionForbidden({
                action: Action.EditMembersEligibility,
                ...props,
              })}
              onEdit={() => setEditPermKind('Member')}
              userKind="member"
            />

            <RoleAndTraitLabels
              roles={grants.roles}
              traits={grants.traitList}
              accessKind="grants"
              toolTipContent={getActionForbiddenInfo({
                action: Action.EditMembersGrants,
                ...props,
              })}
              editDisabled={isActionForbidden({
                action: Action.EditMembersGrants,
                ...props,
              })}
              onEdit={() => setEditPermKind('Grants')}
              required
              userKind="member"
            />

            {/* inherited  permissions from nested access list */}
            {(inheritedMemberRoles?.length > 0 ||
              inheritedMemberTraits?.length > 0) && (
              <RoleAndTraitLabels
                roles={inheritedMemberRoles}
                traits={inheritedMemberTraits}
                required
                accessKind="inherited"
                toolTipContent=""
                editDisabled={false}
                userKind="member"
              />
            )}
          </Flex>
        </Flex>
      </Flex>
      {canReadMembers && (
        <>
          <Box textAlign="right">
            <HoverTooltip
              tipContent={getActionForbiddenInfo({
                action: Action.EditMembers,
                ...props,
              })}
            >
              <ButtonText
                disabled={isActionForbidden({
                  action: Action.EditMembers,
                  ...props,
                })}
                onClick={() => setShowEnrollNewMembers(true)}
                gap={2}
                fill="border"
              >
                <Add size="small" />
                Add New Members or Access Lists
              </ButtonText>
            </HoverTooltip>
          </Box>
          <AccessListMemberTable
            accessList={accessList}
            isReadOnlyOktaList={isReadOnlyOktaList}
            members={members}
            perms={perms}
            onDeleteMember={setDeleteMember}
          />
        </>
      )}

      {showEnrollNewMembers && (
        <EnrollNewMembers
          onClose={() => setShowEnrollNewMembers(false)}
          accessList={accessList}
          updateAccessList={updateAccessList}
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
      {editPermKind && (
        <EditEligibilityOrGrantRoles
          onClose={() => setEditPermKind(null)}
          editKind={editPermKind}
          updateAccessList={updateAccessList}
          accessList={accessList}
        />
      )}
    </Box>
  );
}

export const AccessListMemberTable = ({
  accessList,
  isReadOnlyOktaList,
  members,
  perms,
  onDeleteMember = null,
  hideIneligibleReason = false,
  isReviewing = false,
}: {
  accessList: Pick<AccessListModified, 'origin' | 'type'>;
  isReadOnlyOktaList?: boolean;
  members: AccessListModified['members'];
  perms?: Perms;
  onDeleteMember?(m: AccessListModified['members'][number]): void;
  hideIneligibleReason?: boolean;
  isReviewing?: boolean;
}) => {
  const onClickNestedList = useOnClickNestedList();

  return (
    <Table
      data={members}
      columns={[
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
                    hideIneligibleReason || title ? `View list '${title}'` : ''
                  }
                >
                  <Flex
                    flexDirection="row"
                    alignItems="center"
                    justifyContent="space-between"
                    gap={1}
                  >
                    <NestedListLink
                      title={
                        hideIneligibleReason || title
                          ? `View list '${title}'`
                          : ''
                      }
                      onClick={() => onClickNestedList(name)}
                      disabled={!hideIneligibleReason && !title}
                    >
                      {title || name}
                    </NestedListLink>
                    {!hideIneligibleReason && !title && (
                      <IconTooltip kind="warning">
                        Insufficient permissions to view Access List
                      </IconTooltip>
                    )}
                  </Flex>
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
          key: 'addedBy',
          headerText: 'Added By',
          isSortable: true,
          render: ({ addedBy, ineligibleReason }) => (
            <CustomCell disabled={!hideIneligibleReason && !!ineligibleReason}>
              {addedBy}
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
          render: ({ joined, ineligibleReason, reason }) => (
            <CustomCell disabled={!hideIneligibleReason && !!ineligibleReason}>
              <HoverTooltip tipContent={reason && `Reason: ${reason}`}>
                <Flex gap={2}>
                  <div>{getFormattedDate(joined)}</div>
                  {reason && <Text>&quot;{reason}&quot;</Text>}
                </Flex>
              </HoverTooltip>
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
          render: member => {
            const accessProps = {
              accessList,
              isReadOnlyOktaList,
              action: Action.EditMembers,
              perms,
            };

            return (
              <UserRevokeButtonCell
                disabled={
                  perms
                    ? isActionForbidden(accessProps)
                    : isScim(accessList.type)
                }
                tooltip={
                  perms ? getActionForbiddenInfo(accessProps) : undefined
                }
                onClick={() => onDeleteMember(member)}
                ineligibleReason={member.ineligibleReason}
                hideIneligibleReason={hideIneligibleReason}
                isReviewing={isReviewing}
              />
            );
          },
        },
      ]}
      emptyText="No Members Found"
      isSearchable
      pagination={{
        pageSize: 10,
        pagerPosition: 'both',
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
            width="180px"
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
