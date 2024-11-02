import React, { useEffect, useState } from 'react';
import { Link } from 'react-router-dom';
import { useLocation, useHistory } from 'react-router';
import styled from 'styled-components';
import { Box, Indicator, Alert, Flex, Button } from 'design';
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
import { ShieldCheck } from 'design/Icon';

import useTeleport from 'e-teleport/useTeleportE';

import cfg from 'e-teleport/config';
import { useAccessListManagementContext } from 'e-teleport/AccessListManagement/AccessListManagementContext';
import { updateAccessListsCache } from 'e-teleport/AccessListManagement/Shared/Shared';

import { NoAccessState } from '../NoAccessState';
import { FeatureLimitBlurb } from '../Shared/FeatureLimitReached';

import { EmptyState } from './EmptyState/EmptyState';
import { AccessCard } from './AccessCard';

import type { Dispatch, SetStateAction, FormEvent } from 'react';
import type {
  AccessList,
  AccessListGrant,
} from 'e-teleport/services/accessmanagement';

export type AccessListWithModifiedGrants = Omit<AccessList, 'grants'> & {
  grants: AccessListGrant & { traitList: string[] };
  ownerGrants: AccessListGrant & { traitList: string[] };
  needsReviewBy: Date | null;
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

  const showCreateBtn =
    attempt.attempt.status === 'success'
      ? !!accessLists.length
      : attempt.attempt.status !== 'processing';
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
          attempt={attempt.attempt}
          accessLists={accessLists}
          searchValue={searchValue}
          setSearchValue={setSearchValue}
        />
      </Box>
      {notificationItem}
    </FeatureBox>
  );
}

const MainContent = ({
  attempt,
  accessLists,
  searchValue,
  setSearchValue,
}: {
  attempt: ReturnType<
    typeof useAccessListManagementContext
  >['attempt']['attempt'];
  accessLists: ReturnType<typeof useAccessListManagementContext>['accessLists'];
  searchValue: string;
  setSearchValue: Dispatch<SetStateAction<string>>;
}) => {
  const history = useHistory();

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

  const filteredAccesses = filterAccessLists(accessLists, searchValue);

  return (
    <>
      <Box width="600px" mb={4}>
        <InputWrapper
          onSubmit={e => handleOnSubmitSearch(e, history, setSearchValue)}
        >
          <StyledInput
            placeholder="Search by title or description"
            max={100}
            defaultValue={searchValue}
            name="searchValue"
          />
        </InputWrapper>
      </Box>
      <AccessListContainer>
        {filteredAccesses.length > 0
          ? filteredAccesses.map(a => (
              <AccessCard
                accessList={a}
                key={a.id}
                onClick={() =>
                  handleOnClickViewAccessList(history, searchValue, a.id)
                }
              />
            ))
          : 'No Access Lists Found'}
      </AccessListContainer>
      {cfg.oss.entitlements.AccessLists.limit !== 0 && (
        <FeatureLimitBlurb limit={cfg.oss.entitlements.AccessLists.limit} />
      )}
    </>
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

const handleOnSubmitSearch = (
  e: FormEvent<HTMLFormElement>,
  history: ReturnType<typeof useHistory>,
  setSearchValue: Dispatch<SetStateAction<string>>
) => {
  const { searchValue } = e.target as typeof e.target & {
    searchValue: { value: string };
  };

  e.preventDefault();
  history.replace(
    encodeUrlQueryParams({
      pathname: location.pathname,
      searchString: searchValue.value,
    })
  );
  setSearchValue(searchValue.value);
};

// filterAccessLists currently only searches through access lists
// "title" and "description".
function filterAccessLists(
  lists: ReturnType<typeof useAccessListManagementContext>['accessLists'],
  searchValue: string
) {
  if (!searchValue?.trim()) {
    return lists;
  }
  // Split the search string into separate words
  // so we can search for each category regardless of order.
  const split = searchValue.split(' ').map(s => s.toLowerCase());
  return lists.filter(r => {
    const title = r.title.toLowerCase();
    const titleMatch = split.every(s => title.includes(s));
    if (titleMatch) {
      return true;
    }

    const description = r.description.toLowerCase();
    const descriptionMatch = split.every(s => description.includes(s));
    if (descriptionMatch) {
      return true;
    }

    const owners = r.owners.map(o => o.name.toLowerCase());
    const ownerMatch = split.every(s => owners.includes(s));
    if (ownerMatch) {
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

const NotificationContainer = styled.div`
  position: absolute;
  top: ${props => props.theme.space[2]}px;
  right: ${props => props.theme.space[5]}px;
`;

const AccessListContainer = styled(Flex)`
  align-items: stretch;
  align-content: flex-start;
  gap: 12px;
  flex: 1 1 0;
  flex-wrap: wrap;
`;

const InputWrapper = styled.form`
  border-radius: ${props => props.theme.radii[5]}px;
  height: 40px;
  border: 1px solid ${props => props.theme.colors.spotBackground[2]};
  &:hover,
  &:focus,
  &:active {
    background: ${props => props.theme.colors.spotBackground[0]};
  }
`;

const StyledInput = styled.input`
  border: none;
  outline: none;
  box-sizing: border-box;
  height: 100%;
  width: 100%;
  transition: all 0.2s;
  color: ${props => props.theme.colors.text.main};
  background: transparent;
  margin-right: ${props => props.theme.space[3]}px;
  margin-bottom: ${props => props.theme.space[2]}px;
  padding: ${props => props.theme.space[3]}px;
`;

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
