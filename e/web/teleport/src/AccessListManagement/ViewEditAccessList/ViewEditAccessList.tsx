import React, { useEffect, useState } from 'react';
import { format } from 'date-fns';
import { useHistory, useLocation, useParams } from 'react-router';
import useAttempt from 'shared/hooks/useAttemptNext';
import { Alert, Box, ButtonSecondary, Flex, H1, Indicator, Text } from 'design';
import { ArrowBack, ArrowForward, ListMagnifyingGlass } from 'design/Icon';
import {
  FeatureBox,
  FeatureHeader,
  FeatureHeaderTitle,
} from 'teleport/components/Layout';
import { OutlineInfo } from 'design/Alert/Alert';

import useTeleport from 'e-teleport/useTeleportE';

import {
  AccessList,
  AccessListMember,
  AccessListMemberKind,
  accessManagementService,
} from 'e-teleport/services/accessmanagement';
import cfg from 'e-teleport/config';
import { useAccessListManagementContext } from 'e-teleport/AccessListManagement/AccessListManagementContext';

import { OktaBadge } from '../Shared/OktaBadge';

import { ReviewAccessList } from './ReviewAccessList';

import { OwnersList } from './Owners/OwnersList';
import { MembersList } from './Members/MembersList';
import { Specs } from './Specs/Specs';
import { DeleteAccessListConfirmDialog } from './DeleteAccessListConfirmDialog';

import {
  ButtonPencil,
  getPerms,
  getTitlesForNestedListOwnersMembers,
  modifyAccessList,
} from './Shared';
import { EditTitle } from './Specs/EditTitle';

import type { AccessListModified, Perms } from './Shared';
import type { UserOption } from 'e-teleport/AccessListManagement/Shared/Shared';
import type { Option } from 'shared/components/Select';
import type { AccessListWithModifiedGrants } from 'e-teleport/AccessListManagement/AccessLists/AccessLists';

const noAccessDeleteMsg = 'You do not have access to delete this access_list';

export function ViewEditAccessList() {
  const ctx = useTeleport();
  const {
    attempt,
    accessLists,
    userOptions,
    fetchRoleOptions,
    fetchUsersAndRoles,
    usersAndRolesAttempt,
    processAccessLists,
  } = useAccessListManagementContext();
  const location = useLocation<{
    startReviewFor?: string;
    previousPaths?: string[];
  }>();
  const history = useHistory();
  const { accessListId } = useParams<{ accessListId: string }>();

  const scopedAttempt = useAttempt('processing');
  const [deleteConfirm, setDeleteConfirm] = useState(false);
  const [accessList, setAccessList] = useState<AccessListModified>();

  const [perms, setPerms] = useState<Perms>(getPerms({}));
  const [reviewing, setReviewing] = useState(false);
  const [showEditTitle, setShowEditTitle] = useState(false);

  function updateAccessList(
    newAccessList: AccessList,
    members?: AccessListMember[]
  ) {
    const [membersCount, memberListCount] = (
      members ||
      accessList.members ||
      []
    ).reduce(
      (acc, m) => [
        acc[0] + (m.membershipKind === AccessListMemberKind.List ? 0 : 1),
        acc[1] + (m.membershipKind === AccessListMemberKind.List ? 1 : 0),
      ],
      [0, 0]
    );

    const [modifiedAccessList, newPerms] = modifyAccessList(
      {
        ...newAccessList,
        // These fields are only calculated on the backend, so their existing state
        // should override the nil values on `newAccessList`.
        inheritedMemberGrants: accessList?.inheritedMemberGrants,
        membersCount,
        memberListCount,
      },
      accessLists,
      ctx
    );
    setAccessList(modifiedAccessList);
    setPerms(newPerms);

    // When an access list is modified, any existing clicked/seen states for it should be reset.
    ctx.storeNotifications.resetStatesForNotification(accessList.id);

    // We also want to update the 'allAccessLists' state with the new access list.
    processAccessLists(prev =>
      prev.map(list =>
        list.id === modifiedAccessList.id ? modifiedAccessList : list
      )
    );
  }

  // When `allAccessLists` changes, we need to set the name of any nested lists.
  useEffect(() => {
    if (
      attempt.attempt.status !== 'success' ||
      !accessList ||
      !accessLists?.length
    ) {
      return;
    }

    const updatedList = {
      ...accessList,
      ...getTitlesForNestedListOwnersMembers(
        {
          members: accessList.members,
          owners: accessList.owners,
        },
        accessLists
      ),
    };
    setAccessList(updatedList);
    // We only want to run if/when `allAccessLists` is re-fetched.
    // eslint-disable-next-line react-hooks/exhaustive-deps
  }, [attempt.attempt.status, !!accessList, accessLists?.length]);

  // If this api call succeeded, user is either an owner or
  // has `access_list` list/read rules defined.
  function fetchAccessList() {
    scopedAttempt.setAttempt({ status: 'processing' });

    accessManagementService
      .fetchAccessList(accessListId)
      .then(fetchedAccessList => {
        const [modifiedAccessList, newPerms] = modifyAccessList(
          fetchedAccessList,
          accessLists,
          ctx
        );
        setAccessList(modifiedAccessList);
        setPerms(newPerms);
        scopedAttempt.setAttempt({ status: 'success' });
      })
      .catch((e: Error) =>
        scopedAttempt.setAttempt({ status: 'failed', statusText: e.message })
      );
  }

  const navigateBackFromList = () => {
    const to = location.state?.previousPaths?.length
      ? location.state.previousPaths[location.state.previousPaths.length - 1]
      : cfg.getAccessListManagementRoute();

    history.push(to, {
      previousPaths: (location.state?.previousPaths || []).slice(0, -1),
    });
  };

  // On initial run, this effect will fetch an access list
  // and the list of users and roles.
  // Users and roles are used as dropdown options.
  useEffect(() => {
    if (usersAndRolesAttempt.status !== 'success') {
      fetchUsersAndRoles();
    }
    // eslint-disable-next-line react-hooks/exhaustive-deps
  }, []);

  // The accessListId can change if a user clicks on a different
  // access list in the notification dropdown.
  useEffect(() => {
    if (location.state?.startReviewFor !== accessListId) {
      setReviewing(false);
    }
    if (!accessList || accessList.id !== accessListId) {
      fetchAccessList();
    }
    // eslint-disable-next-line react-hooks/exhaustive-deps
  }, [accessListId]);

  if (
    reviewing ||
    (location.state?.startReviewFor &&
      location.state.startReviewFor === accessListId &&
      attempt.attempt.status === 'success' &&
      !!accessList)
  ) {
    return (
      <ReviewAccessList
        reviewer={ctx.storeUser.getUsername()}
        accessList={accessList}
        fetchRoleOptions={fetchRoleOptions}
        cancelReview={() => {
          setReviewing(false);

          // If we're coming from list of all ALs, we should navigate back there.
          if (location.state?.startReviewFor === accessListId) {
            navigateBackFromList();
            return;
          }

          history.push(location.pathname, {
            startReviewFor: undefined,
            previousPaths: location.state.previousPaths,
          });
        }}
        isOwner={perms.isOwner}
      />
    );
  }

  return (
    <FeatureBox>
      <FeatureHeader alignItems="center" justifyContent="space-between">
        {/* Note: FeatureHeaderTitle normally inserts an H1 element, we're gonna
            do it ourselves instead. */}
        <FeatureHeaderTitle as="div">
          <Flex alignItems="center">
            <div
              data-testid="back-button"
              css={{ display: 'flex', alignItems: 'center', cursor: 'pointer' }}
              onClick={navigateBackFromList}
              aria-label="Back"
              role="button"
              tabIndex={0}
            >
              <ArrowBack mr={2} size="large" color="text.main" />
            </div>
            <FeatureTitle
              perms={perms}
              attempt={scopedAttempt.attempt}
              accessList={accessList}
              setShowEditTitle={setShowEditTitle}
            />
          </Flex>
        </FeatureHeaderTitle>
        {accessList && (
          <ButtonSecondary
            onClick={() => setDeleteConfirm(true)}
            title={perms.adminWhoCanDelete ? '' : noAccessDeleteMsg}
            disabled={
              scopedAttempt.attempt.status === 'processing' ||
              !perms.adminWhoCanDelete
            }
          >
            Delete
          </ButtonSecondary>
        )}
      </FeatureHeader>
      <MainContent
        perms={perms}
        attempt={scopedAttempt.attempt}
        accessList={accessList}
        userOptions={userOptions}
        accessLists={accessLists}
        setReviewing={setReviewing}
        fetchRoleOptions={fetchRoleOptions}
        updateAccessList={updateAccessList}
      />
      {deleteConfirm && (
        <DeleteAccessListConfirmDialog
          isOkta={accessList.isOkta}
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

const FeatureTitle = ({
  perms,
  attempt,
  accessList,
  setShowEditTitle,
}: {
  accessList: AccessListModified;
  attempt: ReturnType<typeof useAttempt>['attempt'];
  perms: Perms;
  setShowEditTitle: (value: boolean) => void;
}) => {
  switch (attempt.status) {
    case 'failed':
      return <>Access List</>;
    case 'success':
      return (
        <Box>
          <Flex alignItems="center" mr={3} gap={1}>
            <H1>{accessList.title}</H1>
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
            <Text typography="body3">{accessList.description}</Text>
          )}
        </Box>
      );
    default:
      return null;
  }
};

const MainContent = ({
  perms,
  attempt,
  userOptions,
  accessList,
  accessLists,
  setReviewing,
  fetchRoleOptions,
  updateAccessList,
}: {
  perms: Perms;
  attempt: ReturnType<typeof useAttempt>['attempt'];
  userOptions: UserOption[];
  accessList: AccessListModified;
  accessLists: AccessListWithModifiedGrants[];
  setReviewing: (value: boolean) => void;
  fetchRoleOptions: (input: string) => Promise<Option[]>;
  updateAccessList: (
    newAccessList: AccessList,
    members?: AccessListMember[]
  ) => void;
}) => {
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

  return (
    <>
      {accessList.requiresReview &&
        (perms.isOwner || perms.adminWhoCanEdit) && (
          <OutlineInfo
            icon={ListMagnifyingGlass}
            primaryAction={{
              content: (
                <>
                  Start Review
                  <ArrowForward size={18} ml={2} />
                </>
              ),
              onClick: () => setReviewing(true),
            }}
          >
            This Access List needs review by{' '}
            {format(accessList.audit.nextDate, 'MM/dd')}.
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
          accessLists={accessLists}
        />
      </Box>
      {(perms.isOwner || perms.adminWhoCanRead) && (
        <MembersList
          canEditMembers={perms.isOwner || perms.adminWhoCanEdit}
          userOptions={userOptions}
          accessList={accessList}
          updateAccessList={updateAccessList}
          accessLists={accessLists}
        />
      )}
    </>
  );
};
