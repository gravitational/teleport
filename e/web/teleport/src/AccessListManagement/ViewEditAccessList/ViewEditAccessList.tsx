import { format } from 'date-fns';
import { useEffect, useState } from 'react';
import { useHistory, useLocation, useParams } from 'react-router';

import { Alert, Box, ButtonSecondary, Flex, H1, Indicator, Text } from 'design';
import { DATE_FORMAT } from 'design/datetime/constants';
import {
  ArrowBack,
  ArrowForward,
  Info,
  ListMagnifyingGlass,
} from 'design/Icon';
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
import {
  FeatureBox,
  FeatureHeader,
  FeatureHeaderTitle,
} from 'teleport/components/Layout';

import { TypeBadge } from '../Shared/TypeBadge';
import { DeleteAccessListConfirmDialog } from './DeleteAccessListConfirmDialog';
import { MembersList } from './Members/MembersList';
import { OwnersList } from './Owners/OwnersList';
import { ReviewAccessList } from './ReviewAccessList';
import {
  ButtonPencil,
  getPerms,
  getTitlesForNestedListOwnersMembers,
  modifyAccessList,
  type AccessListModified,
  type Perms,
} from './Shared';
import { EditTitle } from './Specs/EditTitle';
import { Specs } from './Specs/Specs';

const genericNoAccessMsg =
  'You do not have permission to edit this Access List';
const noAccessDeleteMsg =
  'You do not have permission to delete this Access List';
const oktaTitleMsg =
  "Editing this Access List's title is disabled; it is managed by Okta";
const readOnlyOktaDeleteMsg =
  'Deleting this Access List is disabled; it is managed by Okta and is read-only in Teleport';

export function ViewEditAccessList() {
  const ctx = useTeleport();
  const {
    attempt,
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

  const scopedAttempt = useAttempt('processing');
  const [deleteConfirm, setDeleteConfirm] = useState(false);
  const [accessList, setAccessList] = useState<AccessListModified>();

  const [perms, setPerms] = useState<Perms>(getPerms({}));
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

  showReview: if (
    location.hash === '#review' &&
    attempt.attempt.status === 'success' &&
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
      <FeatureHeader alignItems="center" justifyContent="space-between">
        {/* Note: FeatureHeaderTitle normally inserts an H1 element, we're gonna
            do it ourselves instead. */}
        <FeatureHeaderTitle as="div">
          <Flex alignItems="center">
            <div
              data-testid="back-button"
              css={{ display: 'flex', alignItems: 'center', cursor: 'pointer' }}
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
              <ArrowBack mr={2} size="large" color="text.main" />
            </div>
            <FeatureTitle
              perms={perms}
              isOktaList={isOktaList}
              attempt={scopedAttempt.attempt}
              accessList={accessList}
              setShowEditTitle={setShowEditTitle}
            />
          </Flex>
        </FeatureHeaderTitle>
        {accessList && (
          <HoverTooltip
            tipContent={
              !perms.adminWhoCanDelete
                ? noAccessDeleteMsg
                : isReadOnlyOktaList
                  ? readOnlyOktaDeleteMsg
                  : undefined
            }
            position="bottom"
          >
            <ButtonSecondary
              onClick={() => setDeleteConfirm(true)}
              disabled={
                scopedAttempt.attempt.status === 'processing' ||
                !perms.adminWhoCanDelete ||
                isReadOnlyOktaList
              }
            >
              Delete
            </ButtonSecondary>
          </HoverTooltip>
        )}
      </FeatureHeader>
      <MainContent
        perms={perms}
        attempt={scopedAttempt.attempt}
        accessList={accessList}
        userOptions={userOptions}
        accessLists={accessLists}
        fetchRoleOptions={fetchRoleOptions}
        isReadOnlyOktaList={isReadOnlyOktaList}
        updateAccessList={updateAccessList}
      />
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
        <Box>
          <Flex alignItems="center" mr={3} gap={1}>
            <H1>{accessList.title}</H1>
            {accessList.origin !== AccessListOrigin.Unspecified && (
              <TypeBadge type={accessList.origin} />
            )}
            <HoverTooltip
              tipContent={
                !perms.adminWhoCanEdit
                  ? genericNoAccessMsg
                  : isOktaList
                    ? oktaTitleMsg
                    : undefined
              }
              position="right"
            >
              <ButtonPencil
                onClick={() => setShowEditTitle(true)}
                disabled={!perms.adminWhoCanEdit || isOktaList}
              />
            </HoverTooltip>
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
  fetchRoleOptions,
  isReadOnlyOktaList,
  updateAccessList,
}: {
  perms: Perms;
  attempt: ReturnType<typeof useAttempt>['attempt'];
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
  if (attempt.status === 'processing') {
    return (
      <Box textAlign="center" m={10}>
        <Indicator />
      </Box>
    );
  }
  if (attempt.status === 'failed') {
    return <Alert kind="outline-danger">{attempt.statusText}</Alert>;
  }
  if (attempt.status !== 'success') {
    return null;
  }

  return (
    <>
      <ReviewBanner
        accessList={accessList}
        isReadOnlyOktaList={isReadOnlyOktaList}
        perms={perms}
      />
      <Box mb={6}>
        <Specs
          fetchRoleOptions={fetchRoleOptions}
          accessList={accessList}
          updateAccessList={updateAccessList}
          canEditSpecs={perms.adminWhoCanEdit}
          isReadOnlyOktaList={isReadOnlyOktaList}
        />
      </Box>
      <Box mb={6}>
        <OwnersList
          canEditOwners={perms.adminWhoCanEdit}
          userOptions={userOptions}
          accessList={accessList}
          updateAccessList={updateAccessList}
          accessLists={accessLists}
          isReadOnlyOktaList={isReadOnlyOktaList}
        />
      </Box>
      {(perms.isOwner || perms.adminWhoCanRead) && (
        <MembersList
          canEditMembers={perms.isOwner || perms.adminWhoCanEdit}
          userOptions={userOptions}
          accessList={accessList}
          updateAccessList={updateAccessList}
          accessLists={accessLists}
          isReadOnlyOktaList={isReadOnlyOktaList}
        />
      )}
    </>
  );
};

const ReviewBanner = ({
  accessList,
  perms,
  isReadOnlyOktaList = false,
}: {
  accessList: AccessListModified;
  perms: Perms;
  isReadOnlyOktaList?: boolean;
}) => {
  const location = useLocation();
  const history = useHistory();

  const canReview = perms.isOwner || perms.adminWhoCanEdit;
  const requiresReview =
    (accessList.requiresReview || accessList.audit.nextDate < new Date()) &&
    !isReadOnlyOktaList;

  if (!requiresReview && canReview && location.hash === '#review') {
    return (
      <Alert kind="neutral" icon={Info}>
        {isReadOnlyOktaList
          ? 'This Access List does not require review; it is managed by Okta and is read-only in Teleport.'
          : `This Access List does not require review until ${format(accessList.audit.nextDate, DATE_FORMAT)}.`}
      </Alert>
    );
  }

  if (requiresReview && canReview) {
    return (
      <Alert
        kind="outline-info"
        icon={ListMagnifyingGlass}
        primaryAction={{
          content: (
            <>
              Start Review
              <ArrowForward size={18} ml={2} />
            </>
          ),
          onClick: () =>
            history.push(`${location.pathname}#review`, location.state),
        }}
      >
        This Access List requires review by{' '}
        {format(accessList.audit.nextDate, DATE_FORMAT)}.
      </Alert>
    );
  }

  return null;
};
