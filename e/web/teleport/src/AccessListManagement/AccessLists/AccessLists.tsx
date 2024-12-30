import { ReactNode, useEffect, useMemo, useRef, useState } from 'react';
import { Link } from 'react-router-dom';
import { useHistory, useLocation } from 'react-router';
import styled from 'styled-components';
import {
  Alert,
  Box,
  Button,
  ButtonBorder,
  Flex,
  Indicator,
  Text,
} from 'design';
import { Notification } from 'shared/components/Notification';
import {
  FeatureBox,
  FeatureHeader,
  FeatureHeaderTitle,
} from 'teleport/components/Layout';
import {
  decodeUrlQueryParam,
  encodeUrlQueryParams,
} from 'teleport/components/hooks/useUrlFiltering';
import {
  ArrowRight,
  Magnifier,
  Refresh,
  ShieldCheck,
  User,
  UserList,
} from 'design/Icon';

import { ViewMode } from 'gen-proto-ts/teleport/userpreferences/v1/unified_resource_preferences_pb';
import Table, { Cell } from 'design/DataTable';
import { HoverTooltip } from 'design/Tooltip';
import { format } from 'date-fns';
import { SortMenu } from 'shared/components/Controls/SortMenu';
import { MultiselectMenu } from 'shared/components/Controls/MultiselectMenu';
import { ViewModeSwitch } from 'shared/components/Controls/ViewModeSwitch';

import { MissingPermissionsTooltip } from 'shared/components/MissingPermissionsTooltip';

import cfg from 'e-teleport/config';
import useTeleport from 'e-teleport/useTeleportE';
import {
  AccessListSort,
  useAccessListManagementContext,
} from 'e-teleport/AccessListManagement/AccessListManagementContext';
import {
  filterAccessLists,
  sortAccessLists,
  updateAccessListsCache,
} from 'e-teleport/AccessListManagement/Shared/Shared';
import {
  AccessList,
  AccessListGrant,
  AccessListMemberKind,
  AccessListType,
} from 'e-teleport/services/accessmanagement';
import {
  AccessCard,
  renderRolesAndTraits,
} from 'e-teleport/AccessListManagement/AccessLists/AccessCard';
import { FeatureLimitBlurb } from 'e-teleport/AccessListManagement/Shared/FeatureLimitReached';
import { EmptyState } from 'e-teleport/AccessListManagement/AccessLists/EmptyState/EmptyState';
import { accessListRequiresReview } from 'e-teleport/stores/storeNotificationsE';

import type { Dispatch, SetStateAction } from 'react';
import type { TableColumn } from 'design/DataTable/types';

export type AccessListWithModifiedGrants = Omit<AccessList, 'grants'> & {
  grants: AccessListGrant & { traitList: string[] };
  ownerGrants: AccessListGrant & { traitList: string[] };
  needsReviewBy: Date | null;
  auditNextDate?: Date;
};

export function AccessLists() {
  const ctx = useTeleport();
  const { attempt, accessLists, processAccessLists } =
    useAccessListManagementContext();

  const history = useHistory();
  const location = useLocation<{
    createdList?: AccessList;
    reviewedAccessList?: AccessList;
    deletedAccessListId?: string;
  }>();
  const searchParams = new URLSearchParams(location.search);
  const [searchValue, setSearchValue] = useState(
    decodeUrlQueryParam(searchParams.get('search') || '')
  );

  const perm = ctx.storeUser.getAccessListAccess();
  const canUpsertAsAdmin = perm.create && perm.edit;
  const canList = perm.list;

  const [notificationItem, setNotificationItem] = useState(() => {
    if (location.state?.reviewedAccessList) {
      return (
        <ReviewedNotifciationItem
          reviewedAccessList={location.state.reviewedAccessList}
          onRemove={() => setNotificationItem(null)}
        />
      );
    }
  });

  useEffect(() => {
    if (
      !(
        location.state?.createdList ||
        location.state?.reviewedAccessList ||
        location.state?.deletedAccessListId
      )
    ) {
      return;
    }

    processAccessLists(lists =>
      updateAccessListsCache(lists, location.state, history)
    );
    // eslint-disable-next-line react-hooks/exhaustive-deps
  }, [location.state]);

  // Show 'create' button if 1. Attempt is succeeded and there are access lists, or 2. Attempt is not processing
  const showCreateBtn =
    attempt.attempt.status === 'success'
      ? !!accessLists.length
      : attempt.attempt.status !== 'processing';
  // Show header if: 1. Attempt is not succeeded yet, or 2. Attempt is succeeded and there are access lists
  const showFeatureHeader =
    attempt.attempt.status === 'success' ? !!accessLists.length : true;
  const noPermToCreate = !canUpsertAsAdmin && attempt.attempt.status === '';

  return (
    <FeatureBox css={{ position: 'relative' }}>
      {showFeatureHeader && (
        <FeatureHeader alignItems="center" justifyContent="space-between">
          <FeatureHeaderTitle>Access Lists</FeatureHeaderTitle>
          {showCreateBtn && (
            <HoverTooltip
              position="bottom"
              tipContent={
                noPermToCreate ? (
                  <MissingPermissionsTooltip
                    missingPermissions={['access_list.create']}
                  />
                ) : null
              }
            >
              <Button
                intent="primary"
                fill="border"
                title={
                  noPermToCreate
                    ? `Only Teleport administrators can create new Access Lists`
                    : ''
                }
                disabled={
                  noPermToCreate || attempt.attempt.status === 'processing'
                }
                width="240px"
                as={Link}
                to={cfg.routes.accessListNew}
              >
                Create New Access List
              </Button>
            </HoverTooltip>
          )}
        </FeatureHeader>
      )}
      <Box>
        {!canList && (
          <Alert kind="info">
            You do not have permission to view Access Lists. You are missing
            role permissions: <code>access_list.list</code>
          </Alert>
        )}
        <MainContent
          searchValue={searchValue}
          setSearchValue={setSearchValue}
        />
      </Box>
      {notificationItem}
    </FeatureBox>
  );
}

function MainContent({
  searchValue,
  setSearchValue,
}: {
  searchValue: string;
  setSearchValue: Dispatch<SetStateAction<string>>;
}) {
  const ctx = useTeleport();
  const history = useHistory();
  const {
    attempt: { attempt },
    accessLists,
    allGrantedRoles,
    allOwners,
    refetchAccessLists,
    view: viewMode,
    setView: setViewMode,
    filters: filterValue,
    setFilters: setFilterValue,
    sort: currentSort,
    setSort: setCurrentSort,
  } = useAccessListManagementContext();

  const currentUsername = ctx.storeUser.getUsername();
  const canList = ctx.storeUser.getAccessListAccess().list;

  const showListTypes = useMemo(
    () =>
      accessLists.some(
        a =>
          a.type === AccessListType.Okta ||
          a.type === AccessListType.AwsIdentityCenter
      ),
    [accessLists]
  );

  const ownerFilterOptions = useMemo(
    () =>
      allOwners
        .slice()
        .sort((a, b) => {
          if (a.membershipKind === b.membershipKind) {
            return a.name.localeCompare(b.name);
          }
          return a.membershipKind === AccessListMemberKind.List ? 1 : -1;
        })
        .reduce<{ value: string; label: ReactNode }[]>((acc, owner, idx) => {
          // Always include the current user as the first option.
          if (idx === 0) {
            acc.push({
              value: currentUsername,
              label: renderFilterOwner(`Me (${currentUsername})`),
            });
          }
          // Skip if the owner == the current user.
          if (owner.name === currentUsername) {
            return acc;
          }
          acc.push({
            value: owner.name,
            label: renderFilterOwner(
              owner.membershipKind === AccessListMemberKind.List
                ? accessLists.find(l => l.id === owner.name)?.title ||
                    owner.name
                : owner.name,
              owner.membershipKind
            ),
          });
          return acc;
        }, []),
    [allOwners, accessLists, currentUsername]
  );

  const filteredAccessLists = useMemo(
    () =>
      filterAccessLists({
        accessLists,
        searchValue,
        filterValue,
      }).map(a => ({ ...a, auditNextDate: a.audit.nextDate })),
    [accessLists, searchValue, filterValue]
  );

  const sortedAccessLists = useMemo(
    () => sortAccessLists(filteredAccessLists, currentSort),
    [filteredAccessLists, currentSort]
  );

  const emptyState = (
    <>
      <EmptyState />
      {cfg.oss.entitlements.AccessLists.limit !== 0 && (
        <FeatureLimitBlurb limit={cfg.oss.entitlements.AccessLists.limit} />
      )}
    </>
  );

  if (attempt.status === '' || !canList) {
    return emptyState;
  }

  if (attempt.status === 'processing') {
    return (
      <Box textAlign="center" m={10}>
        <Indicator />
      </Box>
    );
  }
  if (attempt.status === 'failed') {
    return <Alert children={attempt.statusText} />;
  }
  if (attempt.status !== 'success') {
    return null;
  }
  if (accessLists.length === 0) {
    return emptyState;
  }

  return (
    <>
      <Box width="600px" mb={3}>
        <DebouncedSearchInput
          onSearch={value => {
            history.replace(
              encodeUrlQueryParams({
                pathname: location.pathname,
                searchString: value,
              })
            );
            setSearchValue(value);
          }}
          placeholder="Search by title, owner, or description"
          initialValue={searchValue}
        />
      </Box>
      <Flex justifyContent="space-between" alignItems="center" mb={3}>
        <Flex justifyContent="flex-start" alignItems="center" gap={2}>
          {showListTypes && (
            <MultiselectMenu
              options={[
                { value: 'teleport', label: 'Teleport' },
                { value: 'okta', label: 'Okta' },
                {
                  value: 'aws-identity-center',
                  label: 'AWS IAM Identity Center',
                },
              ]}
              onChange={sources => setFilterValue({ source: sources })}
              selected={filterValue.source || []}
              label="List Type"
              tooltip="Filter by type"
            />
          )}
          <MultiselectMenu
            options={ownerFilterOptions}
            onChange={owners => setFilterValue({ owners })}
            selected={filterValue.owners || []}
            label="Owner"
            tooltip="Filter by owner"
          />
          <MultiselectMenu
            options={allGrantedRoles.map(role => ({
              value: role,
              label: role.length > 23 ? `${role.slice(0, 20)}...` : role,
            }))}
            onChange={roles => setFilterValue({ roles })}
            selected={filterValue.roles || []}
            label="Role"
            tooltip="Filter by granted roles"
          />
        </Flex>
        <Flex justifyContent="flex-end" alignItems="center" gap={2}>
          <RefreshButton onRefresh={() => refetchAccessLists(false)} />
          <ViewModeSwitch
            currentViewMode={viewMode}
            setCurrentViewMode={setViewMode}
          />
          <SortMenu
            current={currentSort}
            onChange={setCurrentSort}
            fields={[
              { value: 'title', label: 'Name' },
              { value: 'membersCount', label: 'Member Users' },
              { value: 'memberListCount', label: 'Member Access Lists' },
              { value: 'auditNextDate', label: 'Next Review' },
            ]}
          />
        </Flex>
      </Flex>
      <AccessListContainer viewMode={viewMode} role="list">
        {viewMode === ViewMode.LIST ? (
          <AccessListTable
            accessLists={sortedAccessLists}
            history={history}
            searchValue={searchValue}
            showListTypes={showListTypes}
            currentSort={currentSort}
            setCurrentSort={setCurrentSort}
          />
        ) : sortedAccessLists.length > 0 ? (
          sortedAccessLists.map(a => (
            <AccessCard
              accessList={a}
              key={a.id}
              onClick={() =>
                history.push(cfg.getAccessListManagementRoute(a.id))
              }
            />
          ))
        ) : (
          'No Access Lists Found'
        )}
      </AccessListContainer>
      {cfg.oss.entitlements.AccessLists.limit !== 0 && (
        <FeatureLimitBlurb limit={cfg.oss.entitlements.AccessLists.limit} />
      )}
    </>
  );
}

const AccessListTable = ({
  accessLists,
  history,
  searchValue,
  showListTypes,
  currentSort,
  setCurrentSort,
}: {
  accessLists: AccessListWithModifiedGrants[];
  history: ReturnType<typeof useHistory>;
  searchValue?: string;
  showListTypes?: boolean;
  currentSort: AccessListSort;
  setCurrentSort: (sort: AccessListSort) => void;
}) => {
  const columns = useMemo(() => {
    const cols: TableColumn<AccessListWithModifiedGrants>[] = [
      {
        headerText: 'Name',
        key: 'title',
        render: acl => {
          const desc = acl.description?.trim();
          const descTruncated =
            desc?.length > 80 ? `${desc.slice(0, 77)}...` : desc;
          const titleTruncated =
            acl.title.length > 40 ? `${acl.title.slice(0, 37)}...` : acl.title;

          return (
            <Cell>
              <Flex flexDirection="column" gap={1}>
                <Text>{titleTruncated}</Text>
                {desc?.length ? (
                  <Text typography="body4" color="text.slightlyMuted">
                    {descTruncated}
                  </Text>
                ) : null}
              </Flex>
            </Cell>
          );
        },
      },
    ];

    if (showListTypes) {
      cols.push({
        headerText: 'Type',
        key: 'type',
        render: acl => (
          <Cell>
            <Text>{friendlyListType(acl.type)}</Text>
          </Cell>
        ),
      });
    }

    cols.push(
      {
        // TODO(kiosion): Modify Table to accept icons for header cols so we can use User/UserList here
        // and save on some space/verbosity.
        headerText: 'Member Users',
        key: 'membersCount',
        onSort: (a, b) => {
          if (a.membersCount === b.membersCount) {
            return 0;
          }

          return a.membersCount > b.membersCount ? 1 : -1;
        },
        render: acl => (
          <Cell>
            <Text>{acl.membersCount || '–'}</Text>
          </Cell>
        ),
      },
      {
        headerText: 'Member Access Lists',
        key: 'memberListCount',
        render: acl => (
          <Cell>
            <Text>{acl.memberListCount || '–'}</Text>
          </Cell>
        ),
      },
      {
        headerText: 'Roles',
        key: 'grants',
        render: acl => (
          <Cell>
            {renderRolesAndTraits({
              roles: acl.grants.roles,
              traits: acl.grants.traitList,
            })}
          </Cell>
        ),
      },
      {
        headerText: 'Next Review',
        key: 'auditNextDate',
        onSort: (a, b) => {
          if (!a.audit?.nextDate && !b.audit?.nextDate) {
            return 0;
          }
          if (!a.audit?.nextDate) {
            return 1;
          }
          if (!b.audit?.nextDate) {
            return -1;
          }
          return a.audit.nextDate.getTime() - b.audit.nextDate.getTime();
        },
        render: acl => (
          <TableAuditNextDateCell accessList={acl} history={history} />
        ),
      }
    );

    return cols;
  }, [showListTypes, history, searchValue]);

  return (
    <Table
      data={accessLists}
      emptyText="No Access Lists Found"
      pagination={{ pageSize: 20, pagerPosition: 'bottom' }}
      isSearchable={false}
      row={{
        onClick: (acl: AccessListWithModifiedGrants) =>
          history.push(cfg.getAccessListManagementRoute(acl.id)),
        getStyle: () => ({
          cursor: 'pointer',
          height: '46px',
        }),
      }}
      columns={columns}
      customSort={{
        fieldName: currentSort.fieldName,
        dir: currentSort.dir,
        onSort: sortType => {
          setCurrentSort(sortType as AccessListSort);
        },
      }}
    />
  );
};

const friendlyListType = (listType: string) => {
  switch (listType) {
    case AccessListType.Okta:
      return 'Okta';
    case AccessListType.AwsIdentityCenter:
      return 'AWS IAM Identity Center';
    default:
      return 'Teleport';
  }
};

const TableAuditNextDateCell = ({
  accessList,
  history,
}: {
  accessList: AccessListWithModifiedGrants;
  history: ReturnType<typeof useHistory>;
}) => {
  if (!accessList.audit?.nextDate) {
    return <Cell></Cell>;
  }

  const requiresReview = accessListRequiresReview({
    todayDate: new Date(Date.now()),
    reviewDate: accessList.audit.nextDate,
  });
  const isOverdue = accessList.audit.nextDate < new Date();
  const formatted = format(accessList.audit.nextDate, 'yyyy/MM/dd');

  if (!requiresReview && !isOverdue) {
    return (
      <Cell>
        <Text>{formatted}</Text>
      </Cell>
    );
  }

  return (
    <Cell>
      <ReviewBadge
        onClick={e => {
          e.stopPropagation();
          history.push(
            `${cfg.getAccessListManagementRoute(accessList.id)}#review`
          );
        }}
        isOverdue={isOverdue}
      >
        {formatted}
        {' - Review Now'}
        <ArrowRight size={16} />
      </ReviewBadge>
    </Cell>
  );
};

const renderFilterOwner = (
  owner: string,
  type: AccessListMemberKind = AccessListMemberKind.User
) => {
  return (
    <Flex gap={2} alignItems="center">
      {type === AccessListMemberKind.List ? (
        <UserList size={14} css={{ marginBottom: '-1px' }} />
      ) : (
        <User size={14} css={{ marginBottom: '-1px' }} />
      )}
      <Text>{owner}</Text>
    </Flex>
  );
};

const NotificationContainer = styled.div`
  position: absolute;
  top: ${props => props.theme.space[2]}px;
  right: ${props => props.theme.space[5]}px;
`;

const AccessListContainer = styled(Flex)<{ viewMode?: ViewMode }>`
  display: ${p => (p.viewMode === ViewMode.LIST ? 'block' : 'grid')};
  grid-template-columns: repeat(auto-fill, minmax(300px, 1fr));
  gap: ${p => p.theme.space[3]}px;
`;

const ReviewBadge = styled.button<{
  isOverdue?: boolean;
}>`
  background-color: ${p =>
    p.isOverdue ? p.theme.colors.error.main : p.theme.colors.warning.main};
  border-radius: ${p => p.theme.radii[2]}px;
  border: none;
  outline: none;
  cursor: pointer;
  display: flex;
  flex-direction: row;
  align-items: center;
  justify-content: space-between;
  width: calc(100% + ${p => p.theme.space[2]}px);
  gap: ${p => p.theme.space[2]}px;
  padding: 0 ${p => p.theme.space[2]}px;
  color: ${p => p.theme.colors.text.primaryInverse};
  white-space: nowrap;
  ${p => p.theme.typography.body3};
  margin-left: -${p => p.theme.space[2]}px;
`;

const InputWrapper = styled.form`
  border-radius: ${props => props.theme.radii[5]}px;
  height: 40px;
  border: 1px solid ${props => props.theme.colors.spotBackground[2]};
  display: flex;
  flex-direction: row;
  align-items: center;
  justify-content: flex-start;
  padding: 0 ${props => props.theme.space[3]}px 0
    ${props => props.theme.space[3]}px;
  background-color: transparent;
  transition:
    background-color 150ms ease,
    border-color 150ms ease;

  &:focus-within,
  &:active {
    border-color: ${p => p.theme.colors.brand};
  }

  &:hover,
  &:focus-within,
  &:active {
    background-color: ${props => props.theme.colors.spotBackground[0]};
  }
`;

const StyledInput = styled.input`
  border: none;
  outline: none;
  box-sizing: border-box;
  height: 100%;
  width: 100%;
  transition: all 200ms ease;
  color: ${props => props.theme.colors.text.main};
  background: transparent;
  padding: ${props => props.theme.space[3]}px ${props => props.theme.space[3]}px
    ${props => props.theme.space[3]}px ${props => props.theme.space[2]}px;
  flex: 1;
`;

const DebouncedSearchInput = ({
  onSearch,
  placeholder = '',
  initialValue = '',
}: {
  onSearch: (searchValue: string) => void;
  placeholder?: string;
  initialValue?: string;
}) => {
  const [searchTerm, setSearchTerm] = useState(initialValue);
  const [debouncedTerm, setDebouncedTerm] = useState('');
  const isFirstRender = useRef(true);

  useEffect(() => {
    const timer = setTimeout(() => {
      setDebouncedTerm(searchTerm);
    }, 350);

    return () => clearTimeout(timer);
  }, [searchTerm]);

  useEffect(() => {
    if (isFirstRender.current && debouncedTerm === '') {
      isFirstRender.current = false;
      return;
    }

    onSearch(debouncedTerm);
    // eslint-disable-next-line react-hooks/exhaustive-deps
  }, [debouncedTerm]);

  return (
    <InputWrapper
      onSubmit={e => {
        e.preventDefault();
        onSearch(searchTerm);
      }}
    >
      <Magnifier size={16} color="text.slightlyMuted" />
      <StyledInput
        placeholder={placeholder}
        autoFocus
        max={100}
        name="searchValue"
        value={searchTerm}
        onChange={e => setSearchTerm(e.target.value)}
      />
    </InputWrapper>
  );
};

const ReviewedNotifciationItem = ({
  reviewedAccessList,
  onRemove,
}: {
  reviewedAccessList: AccessList;
  onRemove(): void;
}) => (
  <NotificationContainer>
    <Notification
      key={reviewedAccessList.id}
      item={{
        id: reviewedAccessList.id,
        severity: 'info',
        content: {
          title: `Submitted review for "${reviewedAccessList.title}"`,
          description: `Next review date is ${reviewedAccessList.audit.nextDate}`,
          icon: ShieldCheck,
        },
      }}
      onRemove={onRemove}
      isAutoRemovable={true}
    />
  </NotificationContainer>
);

const RefreshButton = ({ onRefresh }: { onRefresh: () => void }) => (
  <HoverTooltip tipContent="Refresh">
    <ButtonBorder
      onClick={onRefresh}
      textTransform="none"
      css={`
        padding: 0 4.5px;
      `}
      size="small"
      aria-label="Refresh"
    >
      <Refresh size={12} />
    </ButtonBorder>
  </HoverTooltip>
);
