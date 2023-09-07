import React, { useEffect, useState } from 'react';
import { useParams } from 'react-router';
import { Link } from 'react-router-dom';
import useAttempt from 'shared/hooks/useAttemptNext';
import { Box, Indicator, Alert, Flex, Text, ButtonSecondary } from 'design';
import { ArrowBack } from 'design/Icon';
import useTeleport from 'teleport/useTeleport';
import {
  FeatureBox,
  FeatureHeader,
  FeatureHeaderTitle,
} from 'teleport/components/Layout';
import { Access } from 'teleport/services/user';

import {
  accessManagementService,
  AccessList,
  AccessListRequires,
  AccessListGrant,
} from 'e-teleport/services/accessmanagement';
import cfg from 'e-teleport/config';

import { useFetchUserAndRoles } from '../useFetchUsersAndRoles';
import { TraitConvenience, convertToTraitConvenience } from '../Traits';

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
};

export function ViewEditAccessList() {
  const ctx = useTeleport();
  const { accessListId } = useParams<{ accessListId: string }>();

  const attemptObj = useAttempt('processing');
  const { setAttempt, attempt } = attemptObj;
  const [deleteConfirm, setDeleteConfirm] = useState(false);
  const [accessList, setAccessList] = useState<AccessListModified>();
  const { userOptions, roleOptions, fetchUsersAndRoles } =
    useFetchUserAndRoles(attemptObj);

  const [editAccess, setEditAccess] = useState<EditAccess>(getEditAccess({}));

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
        };
        setAccessList(modifiedAccessList);

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

  useEffect(() => {
    fetchAccessList(true).then(success => success && fetchUsersAndRoles());
  }, []);

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

type EditAccessMeta = {
  hasAccess: boolean;
  // Hover titles for buttons.
  // Can be empty if user hasAccess == true.
  btnTitle: string;
};

// EditAccess determines what kinds of editing actions the viewing
// user can take on this access list.
export type EditAccess = {
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
}) {
  const genericNoAccessListMsg =
    'You do not have access to create and update an access_list';

  const isAdmin = accessListAccess?.create && accessListAccess?.edit;
  const canDeleteAccessList = accessListAccess?.remove;

  return {
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
