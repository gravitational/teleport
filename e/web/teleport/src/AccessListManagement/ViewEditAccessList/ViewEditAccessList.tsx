import React, { useEffect, useState } from 'react';
import { format } from 'date-fns';
import styled from 'styled-components';
import { useParams, useLocation, useHistory } from 'react-router';
import { Link } from 'react-router-dom';
import useAttempt from 'shared/hooks/useAttemptNext';
import {
  Box,
  Indicator,
  Alert,
  Flex,
  Text,
  ButtonSecondary,
  ButtonBorder,
} from 'design';
import { ArrowBack, ListMagnifyingGlass, ArrowForward } from 'design/Icon';
import { fade } from 'design/theme/utils/colorManipulator';
import {
  FeatureBox,
  FeatureHeader,
  FeatureHeaderTitle,
} from 'teleport/components/Layout';
import { Access } from 'teleport/services/user';

import useTeleport from 'e-teleport/useTeleportE';

import {
  accessManagementService,
  AccessList,
  AccessListRequires,
  AccessListGrant,
} from 'e-teleport/services/accessmanagement';
import cfg from 'e-teleport/config';
import { accessListRequiresReview } from 'e-teleport/stores/storeNotificationsE';

import { useFetchUserAndRoles } from '../useFetchUsersAndRoles';
import { TraitConvenience, convertToTraitConvenience } from '../Traits';

import { ReviewAccessList } from './ReviewAccessList';

import { OwnersList } from './Owners/OwnersList';
import { MembersList } from './Members/MembersList';
import { Specs } from './Specs/Specs';
import { DeleteAccessListConfirmDialog } from './DeleteAccessListConfirmDialog';

export type AccessListRequiresWithTraitConvenience = AccessListRequires &
  TraitConvenience;

export type AccessListGrantWithTraitConvenience = AccessListGrant &
  TraitConvenience;

export type AccessListModified = AccessList & {
  membershipRequires: AccessListRequiresWithTraitConvenience;
  ownershipRequires: AccessListRequiresWithTraitConvenience;
  grants: AccessListGrantWithTraitConvenience;
  requiresReview: boolean;
};

export function ViewEditAccessList() {
  const ctx = useTeleport();
  const { accessListId } = useParams<{ accessListId: string }>();
  const loc = useLocation<{ reviewed: boolean }>();
  const history = useHistory();

  const attemptObj = useAttempt('processing');
  const { setAttempt, attempt } = attemptObj;
  const [deleteConfirm, setDeleteConfirm] = useState(false);
  const [accessList, setAccessList] = useState<AccessListModified>();
  const { userOptions, roleOptions, fetchUsersAndRoles } =
    useFetchUserAndRoles(attemptObj);

  const [editAccess, setEditAccess] = useState<EditAccess>(getEditAccess({}));
  const [reviewing, setReviewing] = useState(false);

  // If this api call succeeded, user is either an owner or
  // has `access_list` list/read rules defined.
  function fetchAccessList(initialFetch = false) {
    setAttempt({ status: 'processing' });

    return accessManagementService
      .fetchAccessList(accessListId)
      .then(fetchedAccessList => {
        const modifiedAccessList: AccessListModified = {
          ...fetchedAccessList,
          grants: {
            ...fetchedAccessList.grants,
            ...convertToTraitConvenience(fetchedAccessList.grants.traits),
          },
          ownershipRequires: {
            ...fetchedAccessList.ownershipRequires,
            ...convertToTraitConvenience(
              fetchedAccessList.ownershipRequires.traits
            ),
          },
          membershipRequires: {
            ...fetchedAccessList.membershipRequires,
            ...convertToTraitConvenience(
              fetchedAccessList.membershipRequires.traits
            ),
          },
          requiresReview: accessListRequiresReview({
            todayDate: new Date(),
            reviewDate: fetchedAccessList.audit.nextDate,
          }),
        };
        setAccessList(modifiedAccessList);
        ctx.storeNotifications.updateOrRemoveAccessListNotification(
          fetchedAccessList
        );

        const accessListAccess = ctx.storeUser.getAccessListAccess();
        const isOwner = fetchedAccessList.owners.some(
          owner => owner.name === ctx.storeUser.getUsername()
        );

        setEditAccess(getEditAccess({ accessListAccess, isOwner }));

        // If it was an intial fetch, there are other fetching
        // that needs to be done so we can't set attempt
        // to success just yet.
        if (!initialFetch) {
          setAttempt({ status: 'success' });
        }
        // To give caller an indication that this call succeeded
        // if there are further promise chaining.
        return true;
      })
      .catch((e: Error) =>
        setAttempt({ status: 'failed', statusText: e.message })
      );
  }

  // On initial run, this effect will fetch an access list
  // and the list of users and roles.
  //
  // Users and roles will be used as dropdown option items.
  //
  // On subsequent runs (when the accessListId changes in the URL),
  // it will only fetch an access list. The list of users and roles
  // are cached since it is unlikely to change and we can save
  // some api calls.
  //
  // The accessListId can change if a user clicks on a different
  // access list in the notification dropdown.
  useEffect(() => {
    setReviewing(false);

    let isInitialFetch = true;
    if (userOptions.length || roleOptions.length) {
      isInitialFetch = false;
    }

    fetchAccessList(isInitialFetch).then(fetchAccessListSuccess => {
      if (fetchAccessListSuccess && isInitialFetch) {
        fetchUsersAndRoles();
      }
    });
  }, [accessListId]);

  useEffect(() => {
    if (!loc.state?.reviewed) {
      return;
    }

    // User has finished reviewing.
    // Re-fetching access list to get the latest.

    history.replace({ state: {} }); // clear state
    setReviewing(false);
    fetchAccessList();
  }, [loc.state]);

  if (reviewing) {
    return (
      <ReviewAccessList
        reviewer={ctx.storeUser.getUsername()}
        accessList={accessList}
        roleOptions={roleOptions}
        cancelReview={() => setReviewing(false)}
      />
    );
  }

  let FeatureTitle;
  let MainContent: React.ReactElement;
  if (attempt.status === 'processing') {
    MainContent = (
      <Box textAlign="center" m={10}>
        <Indicator />
      </Box>
    );
  } else if (attempt.status === 'failed') {
    // When fetching fails, it looks weird for the title to be empty.
    FeatureTitle = <>Access List</>;
    MainContent = <Alert children={attempt.statusText} />;
  } else if (attempt.status === 'success') {
    FeatureTitle = (
      <Box>
        <Text
          fontSize={5}
          mr={3}
          mb={accessList.description ? 2 : 0}
          css={{ lineHeight: '21px' }}
        >
          {accessList.title}
        </Text>
        {accessList.description && (
          <Text fontSize={1} css={{ lineHeight: '12px' }}>
            {accessList.description}
          </Text>
        )}
      </Box>
    );

    MainContent = (
      <>
        {accessList.requiresReview && editAccess.isOwnerOrAdmin && (
          <ReviewBanner>
            <Flex alignItems="center">
              <ReviewBannerIcon size={18} />
              This Access List needs review by{' '}
              {format(accessList.audit.nextDate, 'MM/dd')}.
            </Flex>
            <ButtonBorder
              justifyContent="space-between"
              onClick={() => setReviewing(true)}
            >
              Start Review
              <ArrowForward size={18} ml={2} />
            </ButtonBorder>
          </ReviewBanner>
        )}
        <Box mb={6}>
          <Specs
            editAccess={editAccess}
            roleOptions={roleOptions}
            accessList={accessList}
            fetchAccessList={fetchAccessList}
          />
        </Box>
        <Box mb={6}>
          <OwnersList
            editAccess={editAccess}
            userOptions={userOptions}
            accessList={accessList}
            fetchAccessList={fetchAccessList}
          />
        </Box>
        {editAccess.members.hasAccess && (
          <MembersList
            editAccess={editAccess}
            userOptions={userOptions}
            accessList={accessList}
            fetchAccessList={fetchAccessList}
          />
        )}
      </>
    );
  }

  return (
    <FeatureBox>
      <FeatureHeader alignItems="center" justifyContent="space-between">
        <FeatureHeaderTitle>
          <Flex alignItems="center">
            <ArrowBack
              as={Link}
              mr={2}
              size="large"
              color="text.main"
              to={cfg.getAccessListManagementRoute()}
            />
            {FeatureTitle}
          </Flex>
        </FeatureHeaderTitle>
        {accessList && (
          <ButtonSecondary
            onClick={() => setDeleteConfirm(true)}
            title={editAccess.deleteList.btnTitle}
            disabled={
              attempt.status === 'processing' ||
              !editAccess.deleteList.hasAccess
            }
          >
            Delete
          </ButtonSecondary>
        )}
      </FeatureHeader>
      {MainContent}
      {deleteConfirm && (
        <DeleteAccessListConfirmDialog
          accessListId={accessList.id}
          accessListName={accessList.title}
          onClose={() => setDeleteConfirm(false)}
        />
      )}
    </FeatureBox>
  );
}

export type EditAccessMeta = {
  hasAccess: boolean;
  // Hover titles for buttons.
  // Can be empty if user hasAccess == true.
  btnTitle?: string;
};

// EditAccess determines what kinds of editing actions the viewing
// user can take on this access list.
//
// TODO: Explore using a PermissionLevel enum instead
// (Owner, Admin, Member) and try to centralize the calculation of
// which type a given user is.
export type EditAccess = {
  isOwnerOrAdmin: boolean;
  // Update owner list and owner eligibility.
  owners: EditAccessMeta;
  // Update member list and member eligility.
  members: EditAccessMeta;
  // Update audit frequency and can "audit" (review) access list.
  audit: EditAccessMeta;
  grants: EditAccessMeta;
  deleteList: EditAccessMeta;
};

function getEditAccess({
  accessListAccess,
  isOwner,
}: {
  accessListAccess?: Access;
  isOwner?: boolean;
}): EditAccess {
  const genericNoAccessListMsg =
    'You do not have access to create and update an access_list';

  const isAdmin = accessListAccess?.create && accessListAccess?.edit;
  const canDeleteAccessList = accessListAccess?.remove;

  return {
    isOwnerOrAdmin: isOwner || isAdmin,
    owners: {
      hasAccess: isAdmin,
      btnTitle: isAdmin ? '' : genericNoAccessListMsg,
    },
    members: {
      hasAccess: isAdmin || isOwner,
      btnTitle: isAdmin || isOwner ? '' : genericNoAccessListMsg,
    },
    audit: {
      hasAccess: isAdmin || isOwner,
      btnTitle: isAdmin || isOwner ? '' : genericNoAccessListMsg,
    },
    grants: {
      hasAccess: isAdmin,
      btnTitle: isAdmin ? '' : genericNoAccessListMsg,
    },
    deleteList: {
      hasAccess: !!canDeleteAccessList,
      btnTitle: canDeleteAccessList
        ? ''
        : 'You do not have access to delete this access_list',
    },
  };
}

const ReviewBanner = styled(Flex)`
  width: 100%;
  border-radius: ${p => p.theme.radii[2]}px;
  border: 2px solid ${p => p.theme.colors.link};
  padding: ${p => p.theme.space[2]}px ${p => p.theme.space[3]}px;
  margin-bottom: ${p => p.theme.space[4]}px;
  background-color: ${p => fade(p.theme.colors.link, 0.1)};
  align-items: center;
  justify-content: space-between;
  font-weight: bold;
`;

const ReviewBannerIcon = styled(ListMagnifyingGlass)`
  background-color: ${p => p.theme.colors.link};
  border-radius: 100px;
  height: 32px;
  width: 32px;
  color: ${p => p.theme.colors.text.primaryInverse};
  margin-right: ${p => p.theme.space[2]}px;
`;
