import React, { useEffect, useState } from 'react';
import { format } from 'date-fns';
import styled from 'styled-components';
import { useParams, useLocation } from 'react-router';
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
import {
  FeatureBox,
  FeatureHeader,
  FeatureHeaderTitle,
} from 'teleport/components/Layout';
import { Access } from 'teleport/services/user';
import { OutlineInfo } from 'design/Alert/Alert';

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
import { OktaBadge } from '../Shared/OktaBadge';

import { ReviewAccessList } from './ReviewAccessList';

import { OwnersList } from './Owners/OwnersList';
import { MembersList } from './Members/MembersList';
import { Specs } from './Specs/Specs';
import { DeleteAccessListConfirmDialog } from './DeleteAccessListConfirmDialog';
import { ButtonPencil } from './Shared';
import { EditTitle } from './Specs/EditTitle';

export type AccessListRequiresWithTraitConvenience = AccessListRequires &
  TraitConvenience;

export type AccessListGrantWithTraitConvenience = AccessListGrant &
  TraitConvenience;

export type AccessListModified = AccessList & {
  membershipRequires: AccessListRequiresWithTraitConvenience;
  ownershipRequires: AccessListRequiresWithTraitConvenience;
  grants: AccessListGrantWithTraitConvenience;
  ownerGrants: AccessListGrantWithTraitConvenience;
  requiresReview: boolean;
};

const noAccessDeleteMsg = 'You do not have access to delete this access_list';

export function ViewEditAccessList() {
  const ctx = useTeleport();
  const location = useLocation<{
    previousPath?: string;
  }>();
  const { accessListId } = useParams<{ accessListId: string }>();

  const attemptObj = useAttempt('processing');
  const { setAttempt, attempt } = attemptObj;
  const [deleteConfirm, setDeleteConfirm] = useState(false);
  const [accessList, setAccessList] = useState<AccessListModified>();
  const { userOptions, fetchRoleOptions, fetchUsersAndRoles } =
    useFetchUserAndRoles(attemptObj);

  const [perms, setPerms] = useState<Perms>(getPerms({}));
  const [reviewing, setReviewing] = useState(false);
  const [showEditTitle, setShowEditTitle] = useState(false);

  function updateAccessList(newAccessList: AccessList) {
    modifyAccessList(newAccessList);

    // When an access list is modified, any existing clicked/seen states for it should be reset.
    ctx.storeNotifications.resetStatesForNotification(accessList.id);
  }

  function modifyAccessList(newAccessList: AccessList) {
    const modifiedAccessList: AccessListModified = {
      ...newAccessList,
      grants: {
        ...newAccessList.grants,
        ...convertToTraitConvenience(newAccessList.grants.traits),
      },
      ownerGrants: {
        ...newAccessList.ownerGrants,
        ...convertToTraitConvenience(newAccessList.ownerGrants.traits),
      },
      ownershipRequires: {
        ...newAccessList.ownershipRequires,
        ...convertToTraitConvenience(newAccessList.ownershipRequires.traits),
      },
      membershipRequires: {
        ...newAccessList.membershipRequires,
        ...convertToTraitConvenience(newAccessList.membershipRequires.traits),
      },
      requiresReview: accessListRequiresReview({
        todayDate: new Date(),
        reviewDate: newAccessList.audit.nextDate,
      }),
    };
    setAccessList(modifiedAccessList);
    ctx.storeNotifications.updateOrRemoveAccessListNotification(newAccessList);

    const accessListAccess = ctx.storeUser.getAccessListAccess();
    const isOwner = newAccessList.owners.some(
      owner => owner.name === ctx.storeUser.getUsername()
    );
    setPerms(getPerms({ accessListAccess, isOwner }));
  }

  // If this api call succeeded, user is either an owner or
  // has `access_list` list/read rules defined.
  function fetchAccessList(initialFetch = false) {
    setAttempt({ status: 'processing' });

    return accessManagementService
      .fetchAccessList(accessListId)
      .then(fetchedAccessList => {
        modifyAccessList(fetchedAccessList);

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
    if (userOptions.length) {
      isInitialFetch = false;
    }

    fetchAccessList(isInitialFetch).then(fetchAccessListSuccess => {
      if (fetchAccessListSuccess && isInitialFetch) {
        fetchUsersAndRoles();
      }
    });
  }, [accessListId]);

  if (reviewing) {
    return (
      <ReviewAccessList
        reviewer={ctx.storeUser.getUsername()}
        accessList={accessList}
        fetchRoleOptions={fetchRoleOptions}
        cancelReview={() => setReviewing(false)}
        isOwner={perms.isOwner}
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
        <Flex alignItems="center" mr={3} gap={1}>
          <Text fontSize={5}>{accessList.title}</Text>
          {accessList.isOkta && <OktaBadge />}
          <ButtonPencil
            title={
              !perms.adminWhoCanEdit
                ? 'You do not have access to edit this access_list'
                : 'Edit Title'
            }
            onClick={() => setShowEditTitle(true)}
            disabled={!perms.adminWhoCanEdit}
          />
        </Flex>
        {accessList.description && (
          <Text fontSize={1} css={{ lineHeight: '12px' }}>
            {accessList.description}
          </Text>
        )}
      </Box>
    );

    MainContent = (
      <>
        {accessList.requiresReview &&
          (perms.isOwner || perms.adminWhoCanEdit) && (
            <OutlineInfo css={{ justifyContent: 'space-between' }}>
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
            </OutlineInfo>
          )}
        <Box mb={6}>
          <Specs
            fetchRoleOptions={fetchRoleOptions}
            accessList={accessList}
            updateAccessList={updateAccessList}
            canEditSpecs={perms.adminWhoCanEdit}
          />
        </Box>
        <Box mb={6}>
          <OwnersList
            canEditOwners={perms.adminWhoCanEdit}
            userOptions={userOptions}
            accessList={accessList}
            updateAccessList={updateAccessList}
          />
        </Box>
        {(perms.isOwner || perms.adminWhoCanRead) && (
          <MembersList
            canEditMembers={perms.isOwner || perms.adminWhoCanEdit}
            userOptions={userOptions}
            accessList={accessList}
            updateAccessList={updateAccessList}
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
              data-testid="back-button"
              as={Link}
              mr={2}
              size="large"
              color="text.main"
              to={
                location.state?.previousPath ||
                cfg.getAccessListManagementRoute()
              }
            />
            {FeatureTitle}
          </Flex>
        </FeatureHeaderTitle>
        {accessList && (
          <ButtonSecondary
            onClick={() => setDeleteConfirm(true)}
            title={perms.adminWhoCanDelete ? '' : noAccessDeleteMsg}
            disabled={
              attempt.status === 'processing' || !perms.adminWhoCanDelete
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
      {showEditTitle && (
        <EditTitle
          onClose={() => setShowEditTitle(false)}
          updateAccessList={updateAccessList}
          accessList={accessList}
        />
      )}
    </FeatureBox>
  );
}

// Perms defines different types of permissions the viewing
// user has.
//
// TODO: Explore using a PermissionLevel enum instead
// (Owner, Admin, Member) and try to centralize the calculation of
// which type a given user is.
export type Perms = {
  adminWhoCanRead: boolean;
  adminWhoCanDelete: boolean;
  adminWhoCanEdit: boolean;
  isOwner: boolean;
};

function getPerms({
  accessListAccess,
  isOwner,
}: {
  accessListAccess?: Access;
  isOwner?: boolean;
}): Perms {
  return {
    isOwner,
    adminWhoCanRead: accessListAccess?.read && accessListAccess?.list,
    adminWhoCanEdit: accessListAccess?.edit,
    adminWhoCanDelete: accessListAccess?.remove,
  };
}

const ReviewBannerIcon = styled(ListMagnifyingGlass)`
  background-color: ${p => p.theme.colors.link};
  border-radius: 100px;
  height: 32px;
  width: 32px;
  color: ${p => p.theme.colors.text.primaryInverse};
  margin-right: ${p => p.theme.space[2]}px;
`;
