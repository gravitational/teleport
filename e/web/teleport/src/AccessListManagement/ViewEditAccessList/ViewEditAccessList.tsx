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
import useAttempt from 'shared/hooks/useAttemptNext';

import { useAccessListManagementContext } from 'e-teleport/AccessListManagement/AccessListManagementContext';
import cfg from 'e-teleport/config';
import {
  AccessListMemberKind,
  AccessListOrigin,
  accessManagementService,
  isReviewable,
  type AccessList,
  type AccessListMember,
} from 'e-teleport/services/accessmanagement';
import useTeleport from 'e-teleport/useTeleportE';
import { FeatureBox } from 'teleport/components/Layout';

import { TextEditKind } from '../Shared/Shared';
import { TypeBadge } from '../Shared/TypeBadge';
import { Action, getActionForbiddenInfo, isActionForbidden } from './access';
import { AuditAndReviews } from './AuditAndReviews/AuditAndReviews';
import { DeleteAccessListConfirmDialog } from './DeleteAccessListConfirmDialog';
import { Members } from './Members/Members';
import { Owners } from './Owners/Owners';
import { ReviewAccessList } from './ReviewAccessList';
import { ReviewBanner } from './ReviewBanner';
import {
  ButtonPencil,
  getPerms,
  modifyAccessList,
  type AccessListModified,
  type Perms,
} from './Shared';
import { EditTitleOrDescription } from './Specs/EditTitleOrDescription';

enum Tab {
  ListMembers = 'tab-members',
  ListOwners = 'tab-owners',
  Audits = 'tab-audits',
}

export function ViewEditAccessList() {
  const ctx = useTeleport();
  const { updateAccessListCache, isOktaPluginReadOnly } =
    useAccessListManagementContext();
  const location = useLocation();
  const history = useHistory();
  const { accessListId } = useParams<{ accessListId: string }>();

  const {
    attempt: fetchViewingAccessListAttempt,
    setAttempt: setFetchViewingAccessListAttempt,
  } = useAttempt('processing');

  const [deleteConfirm, setDeleteConfirm] = useState(false);
  const [accessList, setAccessList] = useState<AccessListModified>();

  const [editKind, setEditKind] = useState<TextEditKind>();

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

    const modifiedAccessList = modifyAccessList({
      ...newAccessList,
      // These fields are only calculated on the backend, so their existing state
      // should override the nil values on `newAccessList`.
      inheritedMemberGrants: accessList?.inheritedMemberGrants,
      membersCount,
      memberListCount,
    });
    setAccessList(modifiedAccessList);
    updateAccessListCache({
      mutationType: 'edited',
      accessList: modifiedAccessList,
    });
  }

  // If this api call succeeded, user is either an owner or
  // has `access_list` list/read rules defined.
  function fetchAccessList() {
    setFetchViewingAccessListAttempt({ status: 'processing' });

    accessManagementService
      .fetchAccessList(accessListId)
      .then(fetchedAccessList => {
        const modifiedAccessList = modifyAccessList(fetchedAccessList);
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

  showReview: if (location.hash === '#review' && !!accessList) {
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
            attempt={fetchViewingAccessListAttempt}
            accessList={accessList}
            setEditKind={setEditKind}
          />
        </Flex>
        {accessList && (
          <HoverTooltip
            tipContent={getActionForbiddenInfo({
              accessList: accessList,
              isReadOnlyOktaList: isReadOnlyOktaList,
              action: Action.Delete,
              perms: perms,
            })}
            placement="left"
          >
            <ButtonSecondary
              onClick={() => setDeleteConfirm(true)}
              disabled={
                fetchViewingAccessListAttempt.status === 'processing' ||
                isActionForbidden({
                  accessList,
                  isReadOnlyOktaList,
                  action: Action.Delete,
                  perms,
                })
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
      {editKind && (
        <EditTitleOrDescription
          onClose={() => setEditKind(null)}
          updateAccessList={updateAccessList}
          accessList={accessList}
          kind={editKind}
        />
      )}
    </FeatureBox>
  );
}

const FeatureTitle = ({
  perms,
  attempt,
  accessList,
  setEditKind,
}: {
  accessList: AccessListModified;
  attempt: ReturnType<typeof useAttempt>['attempt'];
  perms: Perms;
  setEditKind: (kind: TextEditKind) => void;
}) => {
  switch (attempt.status) {
    case 'failed':
      return <>Access List</>;
    case 'success':
      const editForbiddenInfo = getActionForbiddenInfo({
        accessList,
        action: Action.EditTitleOrDescription,
        perms,
      });
      const isEditButtonDisabled = isActionForbidden({
        accessList,
        action: Action.EditTitleOrDescription,
        perms,
      });
      return (
        <Flex flexDirection="column">
          <Flex alignItems="center" mr={3} gap={1}>
            <H1>{accessList.title}</H1>
            {accessList.origin !== AccessListOrigin.Unspecified && (
              <TypeBadge type={accessList.origin} />
            )}
            <FeatureTitleEditButton
              tipContent={editForbiddenInfo}
              isDisabled={isEditButtonDisabled}
              onClick={() => setEditKind('Title')}
              dataTestId="btn-title"
            />
          </Flex>
          {(accessList.description || !isEditButtonDisabled) && (
            <Flex alignItems="center">
              {accessList.description ? (
                <Text
                  fontSize={3}
                  color="text.slightlyMuted"
                  style={{ whiteSpace: 'pre-wrap' }}
                >
                  {accessList.description}
                </Text>
              ) : (
                <Text
                  fontSize={3}
                  color="text.muted"
                  style={{ fontStyle: 'italic' }}
                >
                  Add a description...
                </Text>
              )}
              <FeatureTitleEditButton
                tipContent={editForbiddenInfo}
                isDisabled={isEditButtonDisabled}
                onClick={() => setEditKind('Description')}
                dataTestId="btn-description"
              />
            </Flex>
          )}
        </Flex>
      );
    default:
      return null;
  }
};

const FeatureTitleEditButton = ({
  tipContent,
  isDisabled,
  onClick,
  dataTestId,
}: {
  tipContent: string | undefined;
  isDisabled: boolean;
  onClick: () => void;
  dataTestId: string;
}) => (
  <HoverTooltip tipContent={tipContent} placement="right">
    <ButtonPencil
      onClick={onClick}
      disabled={isDisabled}
      dataTestId={dataTestId}
    />
  </HoverTooltip>
);

const MainContent = ({
  perms,
  accessList,
  isReadOnlyOktaList,
  updateAccessList,
}: {
  perms: Perms;
  accessList: AccessListModified;
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
        {isReviewable(accessList.type) && (
          <TabContainer
            data-tab-id={Tab.Audits}
            selected={activeTab === Tab.Audits}
            onClick={() => setActiveTab(Tab.Audits)}
          >
            Audits
          </TabContainer>
        )}
        <TabBorder ref={borderRef} />
      </TabsContainer>

      {activeTab === Tab.ListOwners && (
        <Owners
          accessList={accessList}
          updateAccessList={updateAccessList}
          isReadOnlyOktaList={isReadOnlyOktaList}
          perms={perms}
        />
      )}

      {activeTab === Tab.ListMembers && (
        <Members
          accessList={accessList}
          updateAccessList={updateAccessList}
          isReadOnlyOktaList={isReadOnlyOktaList}
          perms={perms}
        />
      )}

      {activeTab === Tab.Audits && (
        <AuditAndReviews
          accessList={accessList}
          perms={perms}
          updateAccessList={updateAccessList}
        />
      )}
    </>
  );
};
