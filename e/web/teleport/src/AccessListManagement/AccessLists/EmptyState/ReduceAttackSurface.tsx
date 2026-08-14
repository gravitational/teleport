import { Box } from 'design';

import { AccessListMemberTable } from 'e-teleport/AccessListManagement/ViewEditAccessList/Members/Members';
import type { AccessListModified } from 'e-teleport/AccessListManagement/ViewEditAccessList/Shared';
import {
  AccessListMemberKind,
  AccessListType,
} from 'e-teleport/services/accessmanagement';

import { mockMembers } from './fixtures';
import { Description, Feature, FeatureProps, Title } from './Shared';

export const ReduceAttackSurface = ({
  active,
  onClick,
  isSliding,
}: FeatureProps) => {
  return (
    <Feature $active={active} onClick={onClick} $isSliding={isSliding}>
      <Title>Reduce attack surface</Title>
      <Description>
        <span css={{ display: 'var(--feature-text-display)' }}>
          Grant, review, and auto-provision privileged access on demand.{' '}
        </span>
        Access automatically expires, reducing risk of breaches.
      </Description>
    </Feature>
  );
};

export const ReduceAttackSurfacePreview = () => {
  const mockedMembers = mockMembers.map(m => ({
    ...m,
    title: m.name,
    membershipKind: AccessListMemberKind.User,
  })) satisfies AccessListModified['members'];

  return (
    <Box css={{ transform: 'var(--feature-preview-scale)' }}>
      <AccessListMemberTable
        accessList={{ type: AccessListType.Default }}
        members={mockedMembers}
        perms={{
          adminWhoCanRead: false,
          adminWhoCanDelete: false,
          adminWhoCanEdit: false,
          isOwner: true,
        }}
        onDeleteMember={() => null}
      />
    </Box>
  );
};
