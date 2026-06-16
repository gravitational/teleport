import { format } from 'date-fns';
import {
  PropsWithChildren,
  useEffect,
  useMemo,
  useState,
  type ComponentProps,
  type Dispatch,
  type SetStateAction,
} from 'react';
import { Link, useNavigate } from 'react-router';
import styled from 'styled-components';

import { Alert, Box, Button, ButtonBorder, Flex, Text } from 'design';
import { Danger, Info } from 'design/Alert';
import { CheckboxInput } from 'design/Checkbox';
import Table, { Cell } from 'design/DataTable';
import type { TableColumn } from 'design/DataTable/types';
import { DATE_FORMAT } from 'design/datetime/constants';
import { ArrowRight, Magnifier, Refresh } from 'design/Icon';
import { ShimmerBox } from 'design/ShimmerBox';
import { HoverTooltip } from 'design/Tooltip';
import { ViewMode } from 'gen-proto-ts/teleport/userpreferences/v1/unified_resource_preferences_pb';
import { SortMenu } from 'shared/components/Controls/SortMenuV2';
import { ViewModeSwitch } from 'shared/components/Controls/ViewModeSwitch';
import { MissingPermissionsTooltip } from 'shared/components/MissingPermissionsTooltip';
import { LoadingSkeleton } from 'shared/components/UnifiedResources/shared/LoadingSkeleton';
import { useInfiniteScroll } from 'shared/hooks/useInfiniteScroll';

import { useAccessListManagementContext } from 'e-teleport/AccessListManagement/AccessListManagementContext';
import {
  AccessCard,
  renderRolesAndTraits,
} from 'e-teleport/AccessListManagement/AccessLists/AccessCard';
import { FeatureLimitBlurb } from 'e-teleport/AccessListManagement/Shared/FeatureLimitReached';
import { useAccessListReviewStatus } from 'e-teleport/AccessListManagement/ViewEditAccessList/Shared';
import cfg from 'e-teleport/config';
import {
  AccessList,
  AccessListGrant,
  AccessListOrigin,
} from 'e-teleport/services/accessmanagement';
import useTeleport from 'e-teleport/useTeleportE';
import {
  FeatureBox,
  FeatureHeader,
  FeatureHeaderTitle,
} from 'teleport/components/Layout';
import { ApiError } from 'teleport/services/api/parseError';

import { EmptyState } from './EmptyState/EmptyState';

export type AccessListWithModifiedGrants = Omit<AccessList, 'grants'> & {
  grants: AccessListGrant & { traitList: string[] };
  ownerGrants: AccessListGrant & { traitList: string[] };
  needsReviewBy: Date | null;
  auditNextDate: Date | null;
};

export function AccessLists() {
  const ctx = useTeleport();
  const {
    isFetching,
    accessLists,
    updateSearchParams,
    error,
    search,
    filtersExist,
  } = useAccessListManagementContext();

  const updateSearchValue = (newValue: string) => {
    updateSearchParams({ search: newValue });
  };

  const perms = ctx.storeUser.getAccessListAccess();
  const canUpsertAsAdmin =
    perms.create && perms.edit && perms.list && perms.read;
  const canList = perms.list && perms.read;
  const canCreate = perms.list && perms.read && perms.create;

  const is403Error = error instanceof ApiError && error.response.status === 403;
  const isOtherError = error && !is403Error;
  const noPermToCreate = !canUpsertAsAdmin && !isFetching;
  const showEmptyState =
    !filtersExist && !isFetching && accessLists.length == 0 && !isOtherError;

  if (showEmptyState) {
    return (
      <FeatureBox
        css={{ position: 'relative', maxWidth: 1800, margin: 'auto' }}
      >
        {!canList && is403Error && (
          <Alert kind="info" marginTop={4}>
            You do not have permission to view Access Lists. You are missing
            role permissions: <code>access_list.read,access_list.list</code>
          </Alert>
        )}
        <EmptyState />
        {canCreate &&
          !is403Error &&
          cfg.oss.entitlements.AccessLists.limit !== 0 && (
            <FeatureLimitBlurb limit={cfg.oss.entitlements.AccessLists.limit} />
          )}
      </FeatureBox>
    );
  }

  return (
    <FeatureBox css={{ position: 'relative', maxWidth: 1800, margin: 'auto' }}>
      <FeatureHeader alignItems="center" justifyContent="space-between">
        <FeatureHeaderTitle>Access Lists</FeatureHeaderTitle>
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
            disabled={noPermToCreate || isFetching}
            width="240px"
            as={Link}
            to={cfg.routes.accessListNew}
          >
            Create New Access List
          </Button>
        </HoverTooltip>
      </FeatureHeader>
      <Box>
        {!canList && is403Error && (
          <Alert kind="info">
            You do not have permission to view Access Lists. You are missing
            role permissions: <code>access_list.read,access_list.list</code>
          </Alert>
        )}
        <MainContent searchValue={search} setSearchValue={updateSearchValue} />
      </Box>
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
  const navigate = useNavigate();
  const {
    accessLists,
    view: viewMode,
    setView: setViewMode,
    filters,
    updateSearchParams,
    error,
    isFetching,
    isError,
    refetch,
    fetchNextPage,
    previousSearchParams,
    hasNextPage,
    isFetchingNextPage,
    backendCacheUnhealthy,
    sort: currentSort,
    isOktaPluginReadOnly,
  } = useAccessListManagementContext();

  useEffect(() => {
    if (previousSearchParams) {
      navigate(
        {
          pathname: location.pathname,
          search: previousSearchParams,
        },
        { replace: true }
      );
    }
  }, [navigate, previousSearchParams]);

  useEffect(() => {
    const urlParams = new URLSearchParams(location.search);
    const allowedParams = ['search', 'sort', 'owners'];
    const paramsToRemove: string[] = [];

    urlParams.forEach((_, key) => {
      if (!allowedParams.includes(key)) {
        paramsToRemove.push(key);
      }
    });

    if (paramsToRemove.length > 0) {
      paramsToRemove.forEach(param => urlParams.delete(param));
      navigate(
        {
          pathname: location.pathname,
          search: urlParams.toString(),
        },
        { replace: true }
      );
    }
    // we only want to cleanse this once
  }, []);

  const { setTrigger } = useInfiniteScroll({
    // to match the Promise<void> requirement of `fetch`, we need to call it like this
    fetch: async () => {
      if (hasNextPage && !isFetchingNextPage && !error) {
        fetchNextPage();
      }
    },
  });

  const currentUserName = ctx.storeUser.getUsername();

  const perms = ctx.storeUser.getAccessListAccess();
  const canCreate = perms.list && perms.read && perms.create;

  const showListTypes = useMemo(
    () =>
      accessLists.some(
        a =>
          a.origin === AccessListOrigin.Okta ||
          a.origin === AccessListOrigin.AwsIdentityCenter ||
          a.origin === AccessListOrigin.EntraID ||
          a.origin === AccessListOrigin.Scim
      ),
    [accessLists]
  );

  const is403Error = error instanceof ApiError && error.response.status === 403;

  return (
    <>
      {/* Only show this info at the top, and not sticky like the rest of the errors. */}
      {backendCacheUnhealthy && (
        <Info>
          Advanced sorting and filtering disabled due to an unhealthy or
          disabled cache.
        </Info>
      )}
      {!isFetching && isError && !is403Error && (
        <ErrorsContainer>
          <DangerWithBackground
            primaryAction={{
              content: 'Retry',
              onClick: refetch,
            }}
          >
            {error.message}
          </DangerWithBackground>
        </ErrorsContainer>
      )}

      <Box width="600px" mb={3}>
        <SearchInput
          onSearch={setSearchValue}
          placeholder="Search by title, owner, role or description"
          initialValue={searchValue}
        />
      </Box>
      <Flex justifyContent="space-between" alignItems="center" mb={3}>
        <Flex justifyContent="flex-start" alignItems="center" gap={2}>
          <Flex as="label" alignItems="center">
            <CheckboxInput
              checked={filters.owners.includes(currentUserName)}
              onChange={val => {
                if (val.target.checked) {
                  updateSearchParams({ owners: [currentUserName] });
                  return;
                }
                updateSearchParams({ owners: [] });
              }}
            />
            <Text pl={2}>
              Only show access lists owned by me ({currentUserName})
            </Text>
          </Flex>
        </Flex>
        <Flex justifyContent="flex-end" alignItems="center" gap={2}>
          <RefreshButton onRefresh={refetch} />
          <ViewModeSwitch
            currentViewMode={viewMode}
            setCurrentViewMode={setViewMode}
          />
          {!backendCacheUnhealthy && (
            <SortMenu
              selectedKey={currentSort.fieldName}
              selectedOrder={currentSort.dir}
              onChange={(key, order) =>
                updateSearchParams({ sort: { fieldName: key, dir: order } })
              }
              items={[
                {
                  key: 'title',
                  label: 'Title',
                  ascendingLabel: 'Title, A - Z',
                  descendingLabel: 'Title, Z - A',
                  ascendingOptionLabel: 'Alphabetical, A - Z',
                  descendingOptionLabel: 'Alphabetical, Z - A',
                  defaultOrder: 'ASC',
                },
                {
                  key: 'auditNextDate',
                  label: 'Next review',
                  ascendingOptionLabel: 'Soonest',
                  descendingOptionLabel: 'Farthest',
                  disableSort: true,
                  defaultOrder: 'ASC',
                },
              ]}
            />
          )}
        </Flex>
      </Flex>
      <AccessListContainer viewMode={viewMode} role="list">
        {viewMode === ViewMode.LIST ? (
          <AccessListTable
            isLoading={isFetching}
            accessLists={accessLists}
            showListTypes={showListTypes}
            isOktaReadOnly={isOktaPluginReadOnly}
          />
        ) : accessLists.length > 0 ? (
          accessLists.map(a => (
            <AccessCard
              key={a.id}
              accessList={a}
              onClick={() => navigate(cfg.getAccessListManagementRoute(a.id))}
            />
          ))
        ) : !isFetching ? (
          'No Access Lists Found'
        ) : (
          ''
        )}
      </AccessListContainer>
      {isFetching && viewMode === ViewMode.LIST && (
        <LoadingSkeleton count={8} Element={<LoadingList />} />
      )}
      {isFetching && viewMode === ViewMode.CARD && (
        <CardsContainer>
          <LoadingSkeleton count={48} Element={<LoadingCard />} />
        </CardsContainer>
      )}
      <div ref={setTrigger} />
      {canCreate && cfg.oss.entitlements.AccessLists.limit !== 0 && (
        <FeatureLimitBlurb limit={cfg.oss.entitlements.AccessLists.limit} />
      )}
    </>
  );
}

const NumCell = ({ children, ...props }: ComponentProps<typeof Cell>) => (
  <Cell
    {...props}
    css={{
      width: 1,
      textAlign: 'right',
    }}
  >
    <Text>{children || '–'}</Text>
  </Cell>
);

const AccessListTable = ({
  accessLists,
  showListTypes,
  isLoading,
  isOktaReadOnly = false,
}: {
  accessLists: AccessListWithModifiedGrants[];
  isLoading: boolean;
  showListTypes?: boolean;
  isOktaReadOnly?: boolean;
}) => {
  const navigate = useNavigate();

  const columns = useMemo(() => {
    const cols: TableColumn<AccessListWithModifiedGrants>[] = [
      {
        headerText: 'Name',
        key: 'title',
        render: acl => {
          const desc = acl.description?.trim();

          return (
            <Cell css={{ maxWidth: '20vw', whiteSpace: 'nowrap' }}>
              <Flex flexDirection="column" gap={1}>
                <Text title={acl.title}>{acl.title}</Text>
                {desc?.length ? (
                  <Text
                    title={desc}
                    typography="body4"
                    color="text.slightlyMuted"
                  >
                    {desc}
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
        headerText: 'Origin',
        key: 'origin',
        render: acl => (
          <Cell>
            <Text>{friendlyListOrigin(acl.origin)}</Text>
          </Cell>
        ),
      });
    }

    cols.push(
      {
        // TODO(kiosion): Modify Table to accept icons for header cols so we can use User/UserList here
        // and save on some space/verbosity.
        headerText: 'Users',
        key: 'membersCount',
        onSort: (a, b) => {
          if (a.membersCount === b.membersCount) {
            return 0;
          }

          return a.membersCount > b.membersCount ? 1 : -1;
        },
        render: acl => <NumCell>{acl.membersCount}</NumCell>,
      },
      {
        headerText: 'Lists',
        key: 'memberListCount',
        render: acl => <NumCell>{acl.memberListCount}</NumCell>,
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
          const aDate = getEffectiveAuditNextDate(a, isOktaReadOnly);
          const bDate = getEffectiveAuditNextDate(b, isOktaReadOnly);

          if (!aDate && !bDate) {
            return 0;
          }
          if (!aDate) {
            return 1;
          }
          if (!bDate) {
            return -1;
          }
          return aDate.getTime() - bDate.getTime();
        },
        render: acl => <TableAuditNextDateCell accessList={acl} />,
      }
    );

    return cols;
  }, [showListTypes, isOktaReadOnly]);

  return (
    <Table
      data={accessLists}
      emptyText="No Access Lists Found"
      infiniteScrollProps={{
        fetchStatus: isLoading ? 'loading' : '',
      }}
      isSearchable={false}
      row={{
        onClick: (acl: AccessListWithModifiedGrants) =>
          navigate(cfg.getAccessListManagementRoute(acl.id)),
        getStyle: () => ({
          cursor: 'pointer',
          height: '46px',
        }),
      }}
      columns={columns}
    />
  );
};

const friendlyListOrigin = (listType: string) => {
  switch (listType) {
    case AccessListOrigin.Okta:
      return 'Okta';
    case AccessListOrigin.AwsIdentityCenter:
      return 'AWS IAM Identity Center';
    case AccessListOrigin.EntraID:
      return 'Entra ID';
    case AccessListOrigin.Scim:
      return 'SCIM';
    default:
      return 'Teleport';
  }
};

const TableAuditNextDateCell = ({
  accessList,
}: {
  accessList: AccessListWithModifiedGrants;
}) => {
  const navigate = useNavigate();
  const { isOktaPluginReadOnly } = useAccessListManagementContext();
  const { canReview, requiresReview } = useAccessListReviewStatus(accessList);
  const auditNextDate = getEffectiveAuditNextDate(
    accessList,
    isOktaPluginReadOnly
  );

  if (!auditNextDate) {
    return <Cell></Cell>;
  }
  const showReviewBadge = canReview && requiresReview;
  const isOverdue = showReviewBadge && auditNextDate < new Date();
  const formatted = format(auditNextDate, DATE_FORMAT);

  if (!showReviewBadge) {
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
          navigate(`${cfg.getAccessListManagementRoute(accessList.id)}#review`);
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

function getEffectiveAuditNextDate(
  accessList: AccessListWithModifiedGrants,
  isOktaReadOnly = false
) {
  if (accessList.origin === AccessListOrigin.Okta && isOktaReadOnly) {
    return null;
  }

  return accessList.auditNextDate;
}

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

const SearchInput = ({
  onSearch,
  placeholder = '',
  initialValue = '',
}: {
  onSearch: (searchValue: string) => void;
  placeholder?: string;
  initialValue?: string;
}) => {
  const [searchTerm, setSearchTerm] = useState(initialValue || '');

  useEffect(() => {
    setSearchTerm(initialValue || '');
  }, [initialValue]);

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

const RefreshButton = ({ onRefresh }: { onRefresh: () => void }) => (
  <HoverTooltip tipContent="Refresh">
    <ButtonBorder
      onClick={onRefresh}
      textTransform="none"
      size="small"
      aria-label="Refresh"
    >
      <Refresh size={12} />
    </ButtonBorder>
  </HoverTooltip>
);

function ErrorsContainer(props: PropsWithChildren<unknown>) {
  return <ErrorBox>{props.children}</ErrorBox>;
}

const ErrorBox = styled(Flex)`
  position: sticky;
  flex-direction: column;
  top: ${props => props.theme.space[3]}px;
  gap: ${props => props.theme.space[1]}px;
  padding-top: ${props => props.theme.space[1]}px;
  padding-bottom: ${props => props.theme.space[3]}px;
  z-index: 1;
`;

const DangerWithBackground = styled(Danger)`
  background: ${props => props.theme.colors.levels.sunken};
`;

export function LoadingCard() {
  const [randomizedSize] = useState(() => ({
    name: randomNum(70, 30),
    members: new Array(randomNum(4, 0)),
  }));

  return (
    <LoadingCardBox alignItems="start" height="79px" p={3}>
      <Flex flex={1} flexDirection="column">
        <Box flex={1} mb={2}>
          {/* Name */}
          <ShimmerBox
            height="20px"
            css={`
              flex-basis: ${randomizedSize.name}%;
            `}
          />
        </Box>
        {/* members */}
        <Flex gap={2}>
          {randomizedSize.members.fill(null).map((_, i) => (
            <ShimmerBox key={i} height="12px" width="60px" />
          ))}
        </Flex>
      </Flex>
    </LoadingCardBox>
  );
}
export function LoadingList() {
  const [randomizedSize] = useState(() => ({
    name: randomNum(40, 20),
    roles: randomNum(80, 50),
    members: randomNum(2, 1),
    memberLists: randomNum(2, 1),
  }));

  return (
    <LoadingListRow alignItems="center" height="46px" px={3}>
      {/* name */}
      <Flex flex="0 0 240px" gap={1} flexDirection="column" pr={3}>
        <ShimmerBox
          height="14px"
          css={`
            width: ${randomizedSize.name}%;
          `}
        />
      </Flex>

      {/* member users column */}
      <Flex flex="0 0 170px" justifyContent="flex-start" pr={3}>
        <ShimmerBox height="12px" width={`${randomizedSize.members * 8}px`} />
      </Flex>

      {/* member access lists */}
      <Flex flex="0 0 140px" justifyContent="flex-start" pr={3}>
        <ShimmerBox
          height="12px"
          width={`${randomizedSize.memberLists * 8}px`}
        />
      </Flex>

      {/* roles */}
      <Flex flex="1" justifyContent="flex-start" pr={3}>
        <ShimmerBox
          height="12px"
          css={`
            width: ${randomizedSize.roles}%;
          `}
        />
      </Flex>

      {/* next review*/}
      <Flex flex="0 0 120px" justifyContent="flex-end">
        <ShimmerBox height="12px" width="80px" />
      </Flex>
    </LoadingListRow>
  );
}
function randomNum(min: number, max: number) {
  return Math.floor(Math.random() * (max - min + 1)) + min;
}

const LoadingCardBox = styled(Flex)`
  border-radius: ${props => props.theme.radii[2]}px;
  border: 2px solid ${props => props.theme.colors.spotBackground[0]};
`;

const LoadingListRow = styled(Flex)`
  border-bottom: 1px solid ${props => props.theme.colors.spotBackground[0]};
  cursor: pointer;

  &:hover {
    background-color: ${props => props.theme.colors.spotBackground[0]};
  }
`;

const CardsContainer = styled(Flex)`
  margin-top: ${p => p.theme.space[3]}px;
  display: grid;
  grid-template-columns: repeat(auto-fill, minmax(400px, 1fr));
  gap: 16px;
  display: grid;
  grid-template-columns: repeat(auto-fill, minmax(300px, 1fr));
  gap: ${p => p.theme.space[3]}px;
`;
