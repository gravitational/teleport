import React, { useEffect, useState } from 'react';
import { Link } from 'react-router-dom';
import styled from 'styled-components';
import useAttempt from 'shared/hooks/useAttemptNext';
import { ButtonPrimary, Box, Indicator, Alert, Flex } from 'design';
import {
  FeatureBox,
  FeatureHeader,
  FeatureHeaderTitle,
} from 'teleport/components/Layout';
import { ApiError } from 'teleport/services/api/parseError';

import { accessListRequiresReview } from 'e-teleport/stores/storeNotificationsE';
import useTeleport from 'e-teleport/useTeleportE';
import {
  accessManagementService,
  AccessList,
  AccessListGrant,
} from 'e-teleport/services/accessmanagement';
import cfg from 'e-teleport/config';

import { NoAccessState } from '../NoAccessState';
import { LimitedPreviewNotice } from '../LimitedPreviewNotice';
import { FeatureLimitBlurb } from '../Shared/FeatureLimitReached';
import { makeTraitLabel } from '../Traits';

import { EmptyState } from './EmptyState/EmptyState';
import { AccessCard } from './AccessCard';

export type AccessListWithModifiedGrants = Omit<AccessList, 'grants'> & {
  grants: AccessListGrant & { traitList: string[] };
  needsReviewBy: Date | null;
};

export function AccessLists() {
  const ctx = useTeleport();
  const perm = ctx.storeUser.getAccessListAccess();
  const canUpsertAsAdmin = perm.create && perm.edit;

  const { attempt, setAttempt } = useAttempt('processing');

  const [accesses, setAccesses] = useState<AccessListWithModifiedGrants[]>([]);
  const [searchValue, setSearchValue] = useState('');
  const [filteredAccesses, setFilteredAccesses] = useState<
    AccessListWithModifiedGrants[]
  >([]);

  useEffect(() => {
    setAttempt({ status: 'processing' });
    accessManagementService
      .fetchAccessLists()
      .then(fetchedLists => {
        setAttempt({ status: 'success' });

        // Update notifications for access lists.
        ctx.storeNotifications.setNotificationsForAccessListsRequiringReview(
          fetchedLists,
          ctx.storeUser.state
        );

        // Process traits.
        const todayDate = new Date();
        const updatedAccessList = fetchedLists.map(r => {
          const traitList = [];
          const definedTraitKeys = Object.keys(r.grants.traits);
          if (definedTraitKeys.length > 0) {
            definedTraitKeys.forEach(key => {
              traitList.push(makeTraitLabel(key, r.grants.traits[key]));
            });
          }
          return {
            ...r,
            grants: { ...r.grants, traitList: traitList.sort() },
            needsReviewBy: accessListRequiresReview({
              todayDate,
              reviewDate: r.audit.nextDate,
            })
              ? r.audit.nextDate
              : null,
          };
        });
        setAccesses(updatedAccessList);
        setFilteredAccesses(updatedAccessList);
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

  // onSearch currently only searchs through access lists
  // "title" and "description".
  // TODO(lisa): Consider separating searching logic from updating state,
  // making it a pure function, and adding a unit test
  function onSearch(s: string) {
    if (!s) {
      setFilteredAccesses(accesses);
    }
    // Split the search string into separate words
    // so we can search for each category regardless of order.
    const splitted = s.split(' ').map(s => s.toLowerCase());
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
    setFilteredAccesses(foundResources);
    setSearchValue(s);
  }

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
          <InputWrapper mb={2}>
            <StyledInput
              placeholder="Search by title or description"
              autoFocus
              value={searchValue}
              onChange={e => onSearch(e.target.value)}
              max={100}
            />
          </InputWrapper>
        </Box>
        <AccessListContainer>
          {filteredAccesses.map(a => (
            <AccessCard accessList={a} key={a.id} />
          ))}
        </AccessListContainer>
        {!cfg.oss.isIgsEnabled && <FeatureLimitBlurb />}
      </>
    );
  }

  const noPermToCreate = !canUpsertAsAdmin && attempt.status === '';
  return (
    <FeatureBox>
      {showFeatureHeader && (
        <>
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
          {attempt.status !== 'failed' && <LimitedPreviewNotice />}
        </>
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

const InputWrapper = styled.div`
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
