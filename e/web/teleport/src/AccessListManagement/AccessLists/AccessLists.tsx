import React, { useEffect, useMemo, useRef, useState } from 'react';
import { Link } from 'react-router-dom';
import { useHistory, useLocation } from 'react-router';
import styled from 'styled-components';
import {
  Alert,
  Box,
  Button,
  ButtonBorder,
  ButtonPrimary,
  ButtonSecondary,
  Flex,
  Indicator,
  Menu,
  MenuItem,
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
  ArrowDown,
  ArrowRight,
  ArrowUp,
  ChevronDown,
  Magnifier,
  Refresh,
  Rows,
  ShieldCheck,
  SquaresFour,
  User,
  UserList,
} from 'design/Icon';

import { ViewMode } from 'gen-proto-ts/teleport/userpreferences/v1/unified_resource_preferences_pb';
import Table, { Cell } from 'design/DataTable';
import { HoverTooltip } from 'shared/components/ToolTip';
import { CheckboxInput } from 'design/Checkbox';
import { format } from 'date-fns';

import cfg from 'e-teleport/config';
import useTeleport from 'e-teleport/useTeleportE';
import {
  AccessListFilters,
  AccessListSort,
  useAccessListManagementContext,
} from 'e-teleport/AccessListManagement/AccessListManagementContext';
import { updateAccessListsCache } from 'e-teleport/AccessListManagement/Shared/Shared';
import {
  AccessList,
  AccessListGrant,
  AccessListMemberKind,
} from 'e-teleport/services/accessmanagement';
import {
  AccessCard,
  renderRolesAndTraits,
} from 'e-teleport/AccessListManagement/AccessLists/AccessCard';
import { NoAccessState } from 'e-teleport/AccessListManagement/NoAccessState';
import { FeatureLimitBlurb } from 'e-teleport/AccessListManagement/Shared/FeatureLimitReached';
import { EmptyState } from 'e-teleport/AccessListManagement/AccessLists/EmptyState/EmptyState';
import { accessListRequiresReview } from 'e-teleport/stores/storeNotificationsE';

import type { Dispatch, ReactNode, SetStateAction } from 'react';
import type { SortDir, TableColumn } from 'design/DataTable/types';

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
    previousPaths?: string[];
  }>();
  const searchParams = new URLSearchParams(location.search);
  const [searchValue, setSearchValue] = useState(
    decodeUrlQueryParam(searchParams.get('search') || '')
  );

  const perm = ctx.storeUser.getAccessListAccess();
  const canUpsertAsAdmin = perm.create && perm.edit;

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
          )}
        </FeatureHeader>
      )}
      <Box>
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

  // TODO(kiosion) The code around these filters really should be rewritten / abstracted out
  const allOwnersParsed = useMemo(
    () =>
      allOwners
        .filter(o => o.name !== ctx.storeUser.getUsername())
        .map(o =>
          o.membershipKind === AccessListMemberKind.List
            ? {
                ...o,
                title: accessLists.find(l => l.id === o.name)?.title || o.name,
              }
            : {
                ...o,
                title: o.name,
              }
        ),
    // eslint-disable-next-line react-hooks/exhaustive-deps
    [allOwners, accessLists]
  );

  const hasOktaLists = useMemo(
    () => accessLists.some(a => a.isOkta),
    [accessLists]
  );
  const currentUsername = ctx.storeUser.getUsername();

  const filteredAccessLists = useMemo(
    () =>
      filterAccessLists({
        accessLists,
        searchValue,
        filterValue,
      }).map(a => ({
        ...a,
        auditNextDate: a.audit.nextDate,
      })),
    [accessLists, searchValue, filterValue]
  );

  const sortedAccessLists = useMemo(
    () => sortAccessLists(filteredAccessLists, currentSort),
    [filteredAccessLists, currentSort]
  );

  if (attempt.status === '') {
    return <NoAccessState />;
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
    return (
      <>
        <EmptyState />
        {cfg.oss.entitlements.AccessLists.limit !== 0 && (
          <FeatureLimitBlurb limit={cfg.oss.entitlements.AccessLists.limit} />
        )}
      </>
    );
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
        />
      </Box>
      <Flex justifyContent="space-between" alignItems="center" mb={3}>
        <Flex justifyContent="flex-start" alignItems="center" gap={2}>
          {hasOktaLists && (
            <MultiselectMenu
              options={[
                { value: 'teleport', label: 'Teleport' },
                { value: 'okta', label: 'Okta' },
              ]}
              onChange={sources => setFilterValue({ source: sources })}
              selected={filterValue.source || []}
              label="List Type"
              tooltip="Filter by type"
            />
          )}
          <MultiselectMenu
            options={[
              {
                value: currentUsername,
                label: renderFilterOwner(`Me (${currentUsername})`),
              },
              ...allOwnersParsed.map(owner => ({
                value: owner.name,
                label: renderFilterOwner(owner.title, owner.membershipKind),
              })),
            ]}
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
            currentSort={currentSort}
            onChange={setCurrentSort}
            sortFields={[
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
            hasOktaLists={hasOktaLists}
            currentSort={currentSort}
            setCurrentSort={setCurrentSort}
          />
        ) : sortedAccessLists.length > 0 ? (
          sortedAccessLists.map(a => (
            <AccessCard
              accessList={a}
              key={a.id}
              onClick={() =>
                handleOnClickViewAccessList(history, searchValue, a.id)
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
  hasOktaLists,
  currentSort,
  setCurrentSort,
}: {
  accessLists: AccessListWithModifiedGrants[];
  history: ReturnType<typeof useHistory>;
  searchValue?: string;
  hasOktaLists?: boolean;
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

    if (hasOktaLists) {
      cols.push({
        headerText: 'Type',
        key: 'isOkta',
        render: acl => (
          <Cell>
            <Text>{acl.isOkta ? 'Okta' : 'Teleport'}</Text>
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
          <TableAuditNextDateCell
            accessList={acl}
            history={history}
            searchValue={searchValue}
          />
        ),
      }
    );

    return cols;
  }, [hasOktaLists, history, searchValue]);

  return (
    <Table
      data={accessLists}
      emptyText="No Access Lists Found"
      pagination={{ pageSize: 20, pagerPosition: 'bottom' }}
      isSearchable={false}
      row={{
        onClick: (acl: AccessListWithModifiedGrants) =>
          handleOnClickViewAccessList(history, searchValue, acl.id),
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

const TableAuditNextDateCell = ({
  accessList,
  history,
  searchValue,
}: {
  accessList: AccessListWithModifiedGrants;
  history: ReturnType<typeof useHistory>;
  searchValue?: string;
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
          history.push(`${cfg.getAccessListManagementRoute(accessList.id)}`, {
            startReviewFor: accessList.id,
            previousPaths: [
              encodeUrlQueryParams({
                pathname: location.pathname,
                searchString: searchValue,
              }),
            ],
          });
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

const handleOnClickViewAccessList = (
  history: ReturnType<typeof useHistory>,
  searchValue: string,
  accessListId: string
) => {
  history.push(cfg.getAccessListManagementRoute(accessListId), {
    previousPaths: [
      encodeUrlQueryParams({
        pathname: location.pathname,
        searchString: searchValue,
      }),
    ],
  });
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

const sortAccessLists = (
  accessLists: AccessListWithModifiedGrants[],
  sort: { fieldName: keyof AccessListWithModifiedGrants; dir: SortDir }
) => {
  // TODO(kiosion): JS sorts lists in-place; this seems to be required for React to re-render predictably.
  return [...accessLists].sort((a, b) => {
    const aVal = a[sort.fieldName];
    const bVal = b[sort.fieldName];

    if (aVal === bVal) {
      // Fall back to sorting by title if the values are equal.
      if (sort.fieldName !== 'title') {
        return sort.dir === 'ASC'
          ? a.title > b.title
            ? 1
            : -1
          : a.title < b.title
            ? 1
            : -1;
      }
      return 0;
    }

    return sort.dir === 'ASC' ? (aVal > bVal ? 1 : -1) : aVal < bVal ? 1 : -1;
  });
};

// filterAccessLists currently only searches through access lists
// "title" and "description".
const filterAccessLists = <T extends AccessListWithModifiedGrants>({
  accessLists,
  searchValue,
  filterValue,
}: {
  accessLists: T[];
  searchValue: string;
  filterValue: AccessListFilters;
}) => {
  // Skip if no filters are set.
  if (
    !accessLists?.length ||
    (!searchValue?.trim() &&
      !filterValue.source?.length &&
      !filterValue.owners?.length &&
      !filterValue.roles?.length)
  ) {
    return accessLists;
  }

  let filtered = accessLists;

  if (searchValue?.trim()) {
    // Split the search string into separate words
    // so we can search for each category regardless of order.
    const split = searchValue.split(' ').map(s => s.toLowerCase());

    filtered = filtered.filter(r => {
      const title = r.title.toLowerCase();
      const titleMatch = split.every(s => title.includes(s));
      if (titleMatch) {
        return true;
      }

      const owners = r.owners
        .map(o => o.name)
        .join('')
        .toLowerCase();
      const ownerMatch = split.every(s => owners.includes(s));
      if (ownerMatch) {
        return true;
      }

      const description = r.description.toLowerCase();
      const descriptionMatch = split.every(s => description.includes(s));
      if (descriptionMatch) {
        return true;
      }

      const strRoles = r.grants.roles.join('').toLowerCase();
      const rolesMatch = split.every(s => strRoles.includes(s));
      if (rolesMatch) {
        return true;
      }

      if (searchValue.toLowerCase().includes('okta') && r.isOkta) {
        return true;
      }
    });
  }

  if (filterValue.source?.length) {
    filtered = filtered.filter(acl => {
      if (filterValue.source.includes('teleport') && !acl.isOkta) {
        return true;
      }
      return filterValue.source.includes('okta') && acl.isOkta;
    });
  }

  if (filterValue.owners?.length) {
    filtered = filtered.filter(acl =>
      filterValue.owners.some(ownerName =>
        acl.owners.some(owner => owner.name === ownerName)
      )
    );
  }

  if (filterValue.roles?.length) {
    filtered = filtered.filter(acl =>
      filterValue.roles.some(role => acl.grants.roles.includes(role))
    );
  }

  return filtered;
};

// TODO(kiosion): Should be unified with the similar sort controls for UnifiedResources view.
// Likewise for 'MultiselectMenu'. Both of these may be useful in other places and should be
// moved to a shared location.
const SortMenu = ({
  currentSort,
  sortFields,
  onChange,
}: {
  currentSort: AccessListSort;
  sortFields: { value: keyof AccessListWithModifiedGrants; label: string }[];
  onChange: (value: AccessListSort) => void;
}) => {
  const [anchorEl, setAnchorEl] = useState<HTMLElement>(null);

  const handleOpen = (event: React.MouseEvent<HTMLButtonElement, MouseEvent>) =>
    setAnchorEl(event.currentTarget);

  const handleClose = () => setAnchorEl(null);

  const handleSelect = (value: (typeof sortFields)[number]['value']) => {
    handleClose();
    onChange({
      fieldName: value,
      dir: currentSort.dir,
    });
  };

  return (
    <Flex textAlign="center">
      <HoverTooltip tipContent={'Sort by'}>
        <ButtonBorder
          css={`
            border-right: none;
            border-top-right-radius: 0;
            border-bottom-right-radius: 0;
            border-color: ${props => props.theme.colors.spotBackground[2]};
          `}
          textTransform="none"
          size="small"
          px={2}
          onClick={handleOpen}
        >
          {sortFields.find(f => f.value === currentSort.fieldName)?.label}
        </ButtonBorder>
      </HoverTooltip>
      <Menu
        popoverCss={() => `margin-top: 36px; margin-left: 28px;`}
        transformOrigin={{
          vertical: 'top',
          horizontal: 'right',
        }}
        anchorOrigin={{
          vertical: 'bottom',
          horizontal: 'right',
        }}
        anchorEl={anchorEl}
        open={Boolean(anchorEl)}
        onClose={handleClose}
      >
        {sortFields.map(({ value, label }) => (
          <MenuItem key={value} onClick={() => handleSelect(value)}>
            {label}
          </MenuItem>
        ))}
      </Menu>
      <HoverTooltip tipContent={'Sort direction'}>
        <ButtonBorder
          onClick={() =>
            onChange({
              fieldName: currentSort.fieldName,
              dir: currentSort.dir === 'ASC' ? 'DESC' : 'ASC',
            })
          }
          textTransform="none"
          css={`
            border-top-left-radius: 0;
            border-bottom-left-radius: 0;
            border-color: ${props => props.theme.colors.spotBackground[2]};
          `}
          size="small"
        >
          {currentSort.dir === 'ASC' ? (
            <ArrowUp size={12} />
          ) : (
            <ArrowDown size={12} />
          )}
        </ButtonBorder>
      </HoverTooltip>
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
}: {
  onSearch: (searchValue: string) => void;
  placeholder?: string;
}) => {
  const [searchTerm, setSearchTerm] = useState('');
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

type MultiselectMenuProps<T> = {
  options: {
    value: T;
    label: string | ReactNode;
    disabled?: boolean;
    disabledTooltip?: string;
  }[];
  selected: T[];
  onChange: (selected: T[]) => void;
  label: string | ReactNode;
  tooltip: string;
  buffered?: boolean;
  showIndicator?: boolean;
};

const MultiselectMenuOptionsContainer = styled(Flex)`
  position: sticky;
  top: 0;
  background-color: ${p => p.theme.colors.levels.elevated};
  z-index: 1;
`;

export const MultiselectMenu = <T extends string>({
  onChange,
  options,
  selected,
  label,
  tooltip,
  buffered = false,
  showIndicator = true,
}: MultiselectMenuProps<T>) => {
  // we have a separate state in the filter so we can select a few different things and then click "apply"
  const [intSelected, setIntSelected] = useState<T[]>([]);
  const [anchorEl, setAnchorEl] = useState<HTMLElement>(null);
  const handleOpen = (
    event: React.MouseEvent<HTMLButtonElement, MouseEvent>
  ) => {
    setIntSelected(selected || []);
    setAnchorEl(event.currentTarget);
  };

  const handleClose = () => {
    setAnchorEl(null);
  };

  // if we cancel, we reset the options to what is already selected in the params
  const cancelUpdate = () => {
    setIntSelected(selected || []);
    handleClose();
  };

  const handleSelect = (value: T) => {
    let newSelected = [...(buffered ? intSelected : selected)];

    if (newSelected.includes(value)) {
      newSelected = newSelected.filter(v => v !== value);
    } else {
      newSelected.push(value);
    }

    (buffered ? setIntSelected : onChange)(newSelected);
  };

  const handleSelectAll = () => {
    (buffered ? setIntSelected : onChange)(
      options.filter(o => !o.disabled).map(o => o.value)
    );
  };

  const handleClearAll = () => {
    (buffered ? setIntSelected : onChange)([]);
  };

  const applyFilters = () => {
    onChange(intSelected);
    handleClose();
  };

  return (
    <Flex textAlign="center" alignItems="center">
      <HoverTooltip tipContent={tooltip}>
        <ButtonSecondary size="small" onClick={handleOpen}>
          {label} {selected?.length > 0 ? `(${selected?.length})` : ''}
          <ChevronDown ml={2} size="small" color="text.slightlyMuted" />
          {selected?.length > 0 && showIndicator && <FiltersExistIndicator />}
        </ButtonSecondary>
      </HoverTooltip>
      <Menu
        popoverCss={() => `margin-top: 36px;`}
        menuListCss={() => `overflow-y: scroll;`}
        transformOrigin={{
          vertical: 'top',
          horizontal: 'left',
        }}
        anchorOrigin={{
          vertical: 'bottom',
          horizontal: 'left',
        }}
        anchorEl={anchorEl}
        open={Boolean(anchorEl)}
        onClose={cancelUpdate}
      >
        <MultiselectMenuOptionsContainer gap={2} p={2}>
          <ButtonSecondary
            size="small"
            onClick={handleSelectAll}
            textTransform="none"
            css={`
              background-color: transparent;
            `}
            px={2}
          >
            Select All
          </ButtonSecondary>
          <ButtonSecondary
            size="small"
            onClick={handleClearAll}
            textTransform="none"
            css={`
              background-color: transparent;
            `}
            px={2}
          >
            Clear All
          </ButtonSecondary>
        </MultiselectMenuOptionsContainer>
        {options.map(opt => {
          const $checkbox = (
            <>
              <CheckboxInput
                type="checkbox"
                // @ts-expect-error assigning ReactNode to checkbox name field
                name={opt.label}
                disabled={opt.disabled}
                onChange={() => {
                  handleSelect(opt.value);
                }}
                id={opt.value}
                checked={(buffered ? intSelected : selected)?.includes(
                  opt.value
                )}
              />
              <Text ml={2} fontWeight={300} fontSize={2}>
                {opt.label}
              </Text>
            </>
          );
          return (
            <MenuItem
              disabled={opt.disabled}
              px={2}
              key={opt.value}
              onClick={() => (!opt.disabled ? handleSelect(opt.value) : null)}
            >
              {opt.disabled && opt.disabledTooltip ? (
                <HoverTooltip tipContent={opt.disabledTooltip}>
                  {$checkbox}
                </HoverTooltip>
              ) : (
                $checkbox
              )}
            </MenuItem>
          );
        })}
        {buffered && (
          <Flex justifyContent="space-between" p={2} gap={2}>
            <ButtonPrimary size="small" onClick={applyFilters}>
              Apply Filters
            </ButtonPrimary>
            <ButtonSecondary
              size="small"
              css={`
                background-color: transparent;
              `}
              onClick={cancelUpdate}
            >
              Cancel
            </ButtonSecondary>
          </Flex>
        )}
      </Menu>
    </Flex>
  );
};

const FiltersExistIndicator = styled.div`
  position: absolute;
  top: -4px;
  right: -4px;
  height: 12px;
  width: 12px;
  background-color: ${p => p.theme.colors.brand};
  border-radius: 50%;
  display: inline-block;
`;

const ViewModeSwitch = ({
  currentViewMode,
  setCurrentViewMode,
}: {
  currentViewMode: ViewMode;
  setCurrentViewMode: (viewMode: ViewMode) => void;
}) => {
  return (
    <ViewModeSwitchContainer
      aria-label="View Mode Switch"
      aria-orientation="horizontal"
      role="radiogroup"
    >
      <HoverTooltip tipContent="Card View">
        <ViewModeSwitchButton
          className={currentViewMode === ViewMode.CARD ? 'selected' : ''}
          onClick={() => setCurrentViewMode(ViewMode.CARD)}
          css={`
            border-right: 1px solid
              ${props => props.theme.colors.spotBackground[2]};
            border-top-left-radius: 4px;
            border-bottom-left-radius: 4px;
          `}
          role="radio"
          aria-label="Card View"
          aria-checked={currentViewMode === ViewMode.CARD}
        >
          <SquaresFour size="small" color="text.main" />
        </ViewModeSwitchButton>
      </HoverTooltip>
      <HoverTooltip tipContent="List View">
        <ViewModeSwitchButton
          className={currentViewMode === ViewMode.LIST ? 'selected' : ''}
          onClick={() => setCurrentViewMode(ViewMode.LIST)}
          css={`
            border-top-right-radius: 4px;
            border-bottom-right-radius: 4px;
          `}
          role="radio"
          aria-label="List View"
          aria-checked={currentViewMode === ViewMode.LIST}
        >
          <Rows size="small" color="text.main" />
        </ViewModeSwitchButton>
      </HoverTooltip>
    </ViewModeSwitchContainer>
  );
};

const ViewModeSwitchContainer = styled.div`
  height: 22px;
  width: 48px;
  border: ${p => p.theme.borders[1]} ${p => p.theme.colors.spotBackground[2]};
  border-radius: ${p => p.theme.radii[2]}px;
  display: flex;

  .selected {
    background-color: ${p => p.theme.colors.spotBackground[1]};

    &:focus-visible,
    &:hover {
      background-color: ${p => p.theme.colors.spotBackground[1]};
    }
  }
`;

const ViewModeSwitchButton = styled.button`
  height: 100%;
  width: 100%;
  overflow: hidden;
  border: none;
  display: flex;
  align-items: center;
  justify-content: center;
  cursor: pointer;
  background-color: transparent;
  outline: none;
  transition: outline-width 150ms ease;

  &:focus-visible {
    outline: ${p => p.theme.borders[1]}
      ${p => p.theme.colors.text.slightlyMuted};
  }

  &:focus-visible,
  &:hover {
    background-color: ${p => p.theme.colors.spotBackground[0]};
  }
`;

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
