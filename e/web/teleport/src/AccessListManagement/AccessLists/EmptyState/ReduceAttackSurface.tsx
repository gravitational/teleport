import { Box } from 'design';

import { AccessListMemberTable } from 'e-teleport/AccessListManagement/ViewEditAccessList/Members/MembersList';
import { AccessListMemberKind } from 'e-teleport/services/accessmanagement';

import { Description, Feature, FeatureProps, Title } from './Shared';
import { mockMembers } from './fixtures';

import type { AccessListModified } from 'e-teleport/AccessListManagement/ViewEditAccessList/Shared';

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
        members={mockedMembers}
        canEditMembers={true}
        onDeleteMember={() => null}
        hideReasonCol={true}
      />
    </Box>
  );
};
