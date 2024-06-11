import React, { useEffect, useState, FormEvent } from 'react';
import { Link } from 'react-router-dom';
import { useLocation, useHistory } from 'react-router';
import styled from 'styled-components';
import useAttempt from 'shared/hooks/useAttemptNext';
import { ButtonPrimary, Box, Indicator, Alert, Flex } from 'design';
import {
  FeatureBox,
  FeatureHeader,
  FeatureHeaderTitle,
} from 'teleport/components/Layout';
import { ApiError } from 'teleport/services/api/parseError';
import { decodeUrlQueryParam } from 'teleport/components/hooks/useUrlFiltering';
import { compareByString } from 'teleport/lib/util';

import { accessListRequiresReview } from 'e-teleport/stores/storeNotificationsE';
import useTeleport from 'e-teleport/useTeleportE';
import {
  accessManagementService,
  AccessList,
  AccessListGrant,
} from 'e-teleport/services/accessmanagement';
import cfg from 'e-teleport/config';

import { NoAccessState } from '../NoAccessState';
import { FeatureLimitBlurb } from '../Shared/FeatureLimitReached';
import { makeTraitLabel } from '../Traits';

import { EmptyState } from './EmptyState/EmptyState';
import { AccessCard } from './AccessCard';

export type AccessListWithModifiedGrants = Omit<AccessList, 'grants'> & {
  grants: AccessListGrant & { traitList: string[] };
  ownerGrants: AccessListGrant & { traitList: string[] };
  needsReviewBy: Date | null;
};

export function AccessLists() {
  const ctx = useTeleport();
  const history = useHistory();
  const location = useLocation<{
    createdList?: AccessList;
    deletedAccessListId?: string;
  }>();
  const searchParams = new URLSearchParams(location.search);

  const perm = ctx.storeUser.getAccessListAccess();
  const canUpsertAsAdmin = perm.create && perm.edit;

  const { attempt, setAttempt } = useAttempt('processing');

  const [accesses, setAccesses] = useState<AccessListWithModifiedGrants[]>([]);
  const [searchValue, setSearchValue] = useState(
    decodeUrlQueryParam(searchParams.get('search') || '')
  );

  useEffect(() => {
    setAttempt({ status: 'processing' });
    accessManagementService
      .fetchAccessLists()
      .then(fetchedLists => {
        setAttempt({ status: 'success' });

        // If a location state was set, user came to this view from
        // either creating or deleting an access list.
        // Because of caching, the list from backend won't be updated
        // right way, so we manually update the list here.
        const { createdList, deletedAccessListId } = location.state || {};
        if (createdList) {
          const foundList = fetchedLists.find(l => l.id === createdList.id);
          if (!foundList) {
            fetchedLists.push(createdList);
          }
        }
        if (deletedAccessListId) {
          fetchedLists = fetchedLists.filter(l => l.id !== deletedAccessListId);
        }

        fetchedLists.sort((a, b) =>
          compareByString(
            a.title.toLocaleLowerCase(),
            b.title.toLocaleLowerCase()
          )
        );

        // Clear loc state afterwards but preserving query.
        history.replace({
          state: {},
          pathname: location.pathname,
          search: location.search,
        });

        // Update notifications for access lists.
        ctx.storeNotifications.setNotificationsForAccessListsRequiringReview(
          fetchedLists,
          ctx.storeUser.state
        );

        // Process traits.
        const todayDate = new Date();
        const updatedAccessLists = fetchedLists.map(r => {
          const memberTraitList = [];
          const memberTraitKeys = Object.keys(r.grants.traits);
          if (memberTraitKeys.length > 0) {
            memberTraitKeys.forEach(key => {
              memberTraitList.push(makeTraitLabel(key, r.grants.traits[key]));
            });
          }
          const ownerTraitList = [];
          const ownerTraitKeys = Object.keys(r.ownerGrants.traits);
          if (ownerTraitKeys.length > 0) {
            ownerTraitKeys.forEach(key => {
              ownerTraitList.push(
                makeTraitLabel(key, r.ownerGrants.traits[key])
              );
            });
          }
          return {
            ...r,
            grants: { ...r.grants, traitList: memberTraitList.sort() },
            ownerGrants: { ...r.ownerGrants, traitList: ownerTraitList.sort() },
            needsReviewBy: accessListRequiresReview({
              todayDate,
              reviewDate: r.audit.nextDate,
            })
              ? r.audit.nextDate
              : null,
          };
        });
        setAccesses(updatedAccessLists);
      })
      .catch((e: Error) => {
        if (e instanceof ApiError) {
          // If error is of type "access denied",
          // then the user is neither a member or owner of access lists
          // or have access_list rbac (aka admin).
          if (e.response.status === 403) {
            setAttempt({ status: '' });
            return;
          }
        }
        setAttempt({ status: 'failed', statusText: e.message });
      });

    // Static data fetched on init.
    // eslint-disable-next-line react-hooks/exhaustive-deps
  }, []);

  const encodeUrlQueryParams = (search: string) => {
    const searchParams = new URLSearchParams({ search }).toString();
    return searchParams
      ? `${location.pathname}?${searchParams}`
      : location.pathname;
  };

  // filterAccessLists currently only searchs through access lists
  // "title" and "description".
  function filterAccessLists() {
    if (!searchValue) {
      return accesses;
    }
    // Split the search string into separate words
    // so we can search for each category regardless of order.
    const splitted = searchValue.split(' ').map(s => s.toLowerCase());
    const foundResources = accesses.filter(r => {
      const title = r.title.toLowerCase();
      const titleMatch = splitted.every(s => title.includes(s));
      if (titleMatch) {
        return true;
      }

      const description = r.description.toLowerCase();
      const descriptionMatch = splitted.every(s => description.includes(s));
      if (descriptionMatch) {
        return true;
      }

      const strRoles = r.grants.roles.join('').toLowerCase();
      const rolesMatch = splitted.every(s => strRoles.includes(s));
      if (rolesMatch) {
        return true;
      }
    });
    return foundResources;
  }

  function handleOnClickViewAccessList(accessListId: string) {
    history.push(cfg.getAccessListManagementRoute(accessListId), {
      previousPath: encodeUrlQueryParams(searchValue),
    });
  }

  function handleOnSubmitSearch(e: FormEvent<HTMLFormElement>) {
    const { searchValue } = e.target as typeof e.target & {
      searchValue: { value: string };
    };

    e.preventDefault(); // prevent form default
    history.replace(encodeUrlQueryParams(searchValue.value));
    setSearchValue(searchValue.value);
  }

  const filteredAccesses = filterAccessLists();

  let MainContent: React.ReactElement;
  let showCreateBtn = true;
  let showFeatureHeader = true;
  if (attempt.status === '') {
    MainContent = <NoAccessState />;
  } else if (attempt.status === 'processing') {
    showCreateBtn = false;
    MainContent = (
      <Box textAlign="center" m={10}>
        <Indicator />
      </Box>
    );
  } else if (attempt.status === 'failed') {
    MainContent = <Alert children={attempt.statusText} />;
  } else if (attempt.status === 'success' && accesses.length === 0) {
    MainContent = (
      <>
        <EmptyState />
        {!cfg.oss.isIgsEnabled && <FeatureLimitBlurb />}
      </>
    );
    showCreateBtn = false;
    showFeatureHeader = false;
  } else {
    MainContent = (
      <>
        <Box width="600px" mb={4}>
          <InputWrapper mb={2} onSubmit={handleOnSubmitSearch}>
            <StyledInput
              placeholder="Search by title or description"
              autoFocus
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
                  onClick={() => handleOnClickViewAccessList(a.id)}
                />
              ))
            : 'No Access Lists Found'}
        </AccessListContainer>
        {!cfg.oss.isIgsEnabled && <FeatureLimitBlurb />}
      </>
    );
  }

  const noPermToCreate = !canUpsertAsAdmin && attempt.status === '';
  return (
    <FeatureBox>
      {showFeatureHeader && (
        <FeatureHeader alignItems="center" justifyContent="space-between">
          <FeatureHeaderTitle>Access Lists</FeatureHeaderTitle>
          {showCreateBtn && (
            <ButtonPrimary
              title={
                noPermToCreate
                  ? `Only Teleport administrators can create new Access Lists`
                  : ''
              }
              disabled={noPermToCreate || attempt.status === 'processing'}
              width="240px"
              as={Link}
              to={cfg.routes.accessListNew}
            >
              Create New Access List
            </ButtonPrimary>
          )}
        </FeatureHeader>
      )}
      <Box>{MainContent}</Box>
    </FeatureBox>
  );
}

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
