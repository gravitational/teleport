import { useEffect, useState } from 'react';
import { useHistory, useLocation, useParams } from 'react-router';

import {
  Alert,
  Box,
  ButtonIcon,
  ButtonSecondary,
  Flex,
  H1,
  Indicator,
  Text,
} from 'design';
import { ArrowBack } from 'design/Icon';
import {
  TabBorder,
  TabContainer,
  TabsContainer,
  useSlidingBottomBorderTabs,
} from 'design/Tabs';
import { HoverTooltip } from 'design/Tooltip';
import type { Option } from 'shared/components/Select';
import useAttempt from 'shared/hooks/useAttemptNext';

import { useAccessListManagementContext } from 'e-teleport/AccessListManagement/AccessListManagementContext';
import type { AccessListWithModifiedGrants } from 'e-teleport/AccessListManagement/AccessLists/AccessLists';
import type { UserOption } from 'e-teleport/AccessListManagement/Shared/Shared';
import cfg from 'e-teleport/config';
import {
  AccessListMemberKind,
  AccessListOrigin,
  accessManagementService,
  type AccessList,
  type AccessListMember,
} from 'e-teleport/services/accessmanagement';
import useTeleport from 'e-teleport/useTeleportE';
import { FeatureBox } from 'teleport/components/Layout';

import { TypeBadge } from '../Shared/TypeBadge';
import { AuditAndReviews } from './AuditAndReviews/AuditAndReviews';
import { DeleteAccessListConfirmDialog } from './DeleteAccessListConfirmDialog';
import { noEditAcessMsg } from './errors';
import { Members } from './Members/Members';
import { Owners } from './Owners/Owners';
import { ReviewAccessList } from './ReviewAccessList';
import { ReviewBanner } from './ReviewBanner';
import {
  ButtonPencil,
  getPerms,
  getTitlesForNestedListOwnersMembers,
  modifyAccessList,
  type AccessListModified,
  type Perms,
} from './Shared';
import { EditTitle } from './Specs/EditTitle';

enum Tab {
  ListMembers = 'tab-members',
  ListOwners = 'tab-owners',
  Audits = 'tab-audits',
}

const noAccessDeleteMsg =
  'You do not have permission to delete this Access List';
const oktaTitleMsg =
  "Editing this Access List's title is disabled; it is managed by Okta";
const readOnlyOktaDeleteMsg =
  'Deleting this Access List is disabled; it is managed by Okta and is read-only in Teleport';

export function ViewEditAccessList() {
  const ctx = useTeleport();
  const {
    attempt: fetchAccessListsAttempt,
    accessLists,
    userOptions,
    fetchRoleOptions,
    fetchUsersAndRoles,
    usersAndRolesAttempt,
    isOktaPluginReadOnly,
    processAccessLists,
  } = useAccessListManagementContext();
  const location = useLocation();
  const history = useHistory();
  const { accessListId } = useParams<{ accessListId: string }>();

  const {
    attempt: fetchViewingAccessListAttempt,
    setAttempt: setFetchViewingAccessListAttempt,
  } = useAttempt('processing');

  const [deleteConfirm, setDeleteConfirm] = useState(false);
  const [accessList, setAccessList] = useState<AccessListModified>();

  const [showEditTitle, setShowEditTitle] = useState(false);

  const isOktaList = accessList?.origin === AccessListOrigin.Okta;

  // An Okta-synced Access List is read-only if bidirectional sync is 'false' or omitted.
  // In this case, updates to the members/owners must be made in Okta, and
  // membership/ownership requirements are disabled.
  const isReadOnlyOktaList = isOktaList && isOktaPluginReadOnly;

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

    const modifiedAccessList = modifyAccessList(
      {
        ...newAccessList,
        // These fields are only calculated on the backend, so their existing state
        // should override the nil values on `newAccessList`.
        inheritedMemberGrants: accessList?.inheritedMemberGrants,
        membersCount,
        memberListCount,
      },
      accessLists
    );
    setAccessList(modifiedAccessList);

    // We also want to update the 'allAccessLists' state with the new access list.
    processAccessLists(prev =>
      prev.map(list =>
        list.id === modifiedAccessList.id ? modifiedAccessList : list
      )
    );
  }

  // When fetched accessLists changes, the nested list titles
  // also need to be updated.
  useEffect(() => {
    if (
      fetchAccessListsAttempt.attempt.status !== 'success' ||
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
    // eslint-disable-next-line react-hooks/exhaustive-deps
  }, [
    fetchAccessListsAttempt.attempt.status,
    !!accessList,
    accessLists?.length,
  ]);

  // If this api call succeeded, user is either an owner or
  // has `access_list` list/read rules defined.
  function fetchAccessList() {
    setFetchViewingAccessListAttempt({ status: 'processing' });

    accessManagementService
      .fetchAccessList(accessListId)
      .then(fetchedAccessList => {
        const modifiedAccessList = modifyAccessList(
          fetchedAccessList,
          accessLists
        );
        setAccessList(modifiedAccessList);
        setFetchViewingAccessListAttempt({ status: 'success' });
      })
      .catch((e: Error) =>
        setFetchViewingAccessListAttempt({
          status: 'failed',
          statusText: e.message,
        })
      );
  }

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
    if (!accessList || accessList.id !== accessListId) {
      fetchAccessList();
    }
    // eslint-disable-next-line react-hooks/exhaustive-deps
  }, [accessListId]);

  const perms = getPerms({
    accessListAccess: ctx.storeUser.getAccessListAccess(),
    accessList,
  });

  showReview: if (
    location.hash === '#review' &&
    fetchAccessListsAttempt.attempt.status === 'success' &&
    !!accessList
  ) {
    const canReview = perms.isOwner || perms.adminWhoCanEdit;
    const requiresReview =
      accessList.requiresReview || accessList.audit.nextDate < new Date();

    if (!requiresReview || !canReview) {
      break showReview;
    }

    return (
      <ReviewAccessList
        reviewer={ctx.storeUser.getUsername()}
        accessList={accessList}
        fetchRoleOptions={fetchRoleOptions}
        isReadOnlyOktaList={isReadOnlyOktaList}
        cancelReview={() => {
          if (!location.key || location.key === 'default') {
            history.replace(location.pathname, location.state);
          } else {
            history.goBack();
          }
        }}
        isOwner={perms.isOwner}
      />
    );
  }

  return (
    <FeatureBox>
      <Flex
        alignItems="center"
        justifyContent="space-between"
        my={3}
        gap={3}
        data-testid="header"
      >
        {/* Note: FeatureHeaderTitle normally inserts an H1 element, we're gonna
            do it ourselves instead. */}
        <Flex alignItems="center" gap={3}>
          <ButtonIcon
            data-testid="back-button"
            onClick={() => {
              // If location.key is unset, or 'default', this is the first history entry in-app in the session.
              if (!location.key || location.key === 'default') {
                history.push(cfg.getAccessListManagementRoute());
              } else {
                history.goBack();
              }
            }}
            aria-label="Back"
            role="button"
            tabIndex={0}
          >
            <ArrowBack size="large" color="text.muted" />
          </ButtonIcon>
          <FeatureTitle
            perms={perms}
            isOktaList={isOktaList}
            attempt={fetchViewingAccessListAttempt}
            accessList={accessList}
            setShowEditTitle={setShowEditTitle}
          />
        </Flex>
        {accessList && (
          <HoverTooltip
            tipContent={
              !perms.adminWhoCanDelete
                ? noAccessDeleteMsg
                : isReadOnlyOktaList
                  ? readOnlyOktaDeleteMsg
                  : undefined
            }
            placement="left"
          >
            <ButtonSecondary
              onClick={() => setDeleteConfirm(true)}
              disabled={
                fetchViewingAccessListAttempt.status === 'processing' ||
                !perms.adminWhoCanDelete ||
                isReadOnlyOktaList
              }
            >
              Delete
            </ButtonSecondary>
          </HoverTooltip>
        )}
      </Flex>
      {fetchViewingAccessListAttempt.status === 'processing' && (
        <Box textAlign="center" m={10}>
          <Indicator />
        </Box>
      )}
      {fetchViewingAccessListAttempt.status === 'failed' && (
        <Alert kind="outline-danger">
          {fetchViewingAccessListAttempt.statusText}
        </Alert>
      )}
      {fetchViewingAccessListAttempt.status === 'success' && (
        <MainContent
          perms={perms}
          accessList={accessList}
          userOptions={userOptions}
          accessLists={accessLists}
          fetchRoleOptions={fetchRoleOptions}
          isReadOnlyOktaList={isReadOnlyOktaList}
          updateAccessList={updateAccessList}
        />
      )}
      {deleteConfirm && (
        <DeleteAccessListConfirmDialog
          isOkta={accessList.origin === AccessListOrigin.Okta}
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
  isOktaList,
}: {
  accessList: AccessListModified;
  attempt: ReturnType<typeof useAttempt>['attempt'];
  perms: Perms;
  setShowEditTitle: (value: boolean) => void;
  isOktaList: boolean;
}) => {
  switch (attempt.status) {
    case 'failed':
      return <>Access List</>;
    case 'success':
      return (
        <Flex flexDirection="column">
          <Flex alignItems="center" mr={3} gap={1}>
            <H1>{accessList.title}</H1>
            {accessList.origin !== AccessListOrigin.Unspecified && (
              <TypeBadge type={accessList.origin} />
            )}
            <HoverTooltip
              tipContent={
                !perms.adminWhoCanEdit
                  ? noEditAcessMsg
                  : isOktaList
                    ? oktaTitleMsg
                    : undefined
              }
              placement="right"
            >
              <ButtonPencil
                onClick={() => setShowEditTitle(true)}
                disabled={!perms.adminWhoCanEdit || isOktaList}
                dataTestId="btn-title"
              />
            </HoverTooltip>
          </Flex>
          {accessList.description && (
            <Text fontSize={3} color="text.slightlyMuted">
              {accessList.description}
            </Text>
          )}
        </Flex>
      );
    default:
      return null;
  }
};

const MainContent = ({
  perms,
  userOptions,
  accessList,
  accessLists,
  fetchRoleOptions,
  isReadOnlyOktaList,
  updateAccessList,
}: {
  perms: Perms;
  userOptions: UserOption[];
  accessList: AccessListModified;
  accessLists: AccessListWithModifiedGrants[];
  fetchRoleOptions: (input: string) => Promise<Option[]>;
  isReadOnlyOktaList: boolean;
  updateAccessList: (
    newAccessList: AccessList,
    members?: AccessListMember[]
  ) => void;
}) => {
  const [activeTab, setActiveTab] = useState(Tab.ListMembers);

  const { borderRef, parentRef } = useSlidingBottomBorderTabs({ activeTab });

  const memberCount = accessList.members.length;
  const ownerCount = accessList.owners.length;

  return (
    <>
      <ReviewBanner
        accessList={accessList}
        isReadOnlyOktaList={isReadOnlyOktaList}
        perms={perms}
      />

      <TabsContainer ref={parentRef} mb={3}>
        <TabContainer
          data-tab-id={Tab.ListMembers}
          selected={activeTab === Tab.ListMembers}
          onClick={() => setActiveTab(Tab.ListMembers)}
        >
          Members {memberCount > 0 ? `(${memberCount})` : ''}
        </TabContainer>
        <TabContainer
          data-tab-id={Tab.ListOwners}
          selected={activeTab === Tab.ListOwners}
          onClick={() => setActiveTab(Tab.ListOwners)}
        >
          Owners {ownerCount > 0 ? `(${ownerCount})` : ''}
        </TabContainer>
        <TabContainer
          data-tab-id={Tab.Audits}
          selected={activeTab === Tab.Audits}
          onClick={() => setActiveTab(Tab.Audits)}
        >
          Audits
        </TabContainer>
        <TabBorder ref={borderRef} />
      </TabsContainer>

      {activeTab === Tab.ListOwners && (
        <Owners
          canEditOwners={perms.adminWhoCanEdit}
          userOptions={userOptions}
          accessList={accessList}
          updateAccessList={updateAccessList}
          accessLists={accessLists}
          isReadOnlyOktaList={isReadOnlyOktaList}
          canEditSpecs={perms.adminWhoCanEdit}
          fetchRoleOptions={fetchRoleOptions}
        />
      )}

      {activeTab === Tab.ListMembers && (
        <Members
          canEditMembers={perms.isOwner || perms.adminWhoCanEdit}
          canReadMembers={perms.isOwner || perms.adminWhoCanRead}
          userOptions={userOptions}
          accessList={accessList}
          updateAccessList={updateAccessList}
          accessLists={accessLists}
          isReadOnlyOktaList={isReadOnlyOktaList}
          canEditSpecs={perms.adminWhoCanEdit}
          fetchRoleOptions={fetchRoleOptions}
        />
      )}

      {activeTab === Tab.Audits && (
        <AuditAndReviews
          canListReviews={perms.isOwner || perms.adminWhoCanEdit}
          accessList={accessList}
          canEditSpecs={perms.adminWhoCanEdit}
          updateAccessList={updateAccessList}
        />
      )}
    </>
  );
};
