import { Box } from 'design';
import styled from 'styled-components';
import Validation from 'shared/components/Validation';

import { ReviewAudit } from 'e-teleport/AccessListManagement/ViewEditAccessList/ReviewAccessList/Summary';
import {
  reviewDayOfMonthOpts,
  reviewFrequencyOpts,
} from 'e-teleport/AccessListManagement/Shared/Audit';

import { Description, Feature, FeatureProps, Title } from './Shared';
import { mockMembers } from './fixtures';

export const AchieveCompliance = ({
  active,
  onClick,
  isSliding,
}: FeatureProps) => {
  return (
    <Feature $active={active} onClick={onClick} $isSliding={isSliding}>
      <Title>Achieve compliance</Title>
      <Description>
        Periodically review access in an automated way to improve security and
        achieve compliance.
      </Description>
    </Feature>
  );
};

export const AchieveCompliancePreview = () => {
  return (
    <PreviewWrapper px={4} pt={4} pb={1}>
      <Validation>
        <ReviewAudit
          disabled={false}
          editedMembershipRequires={{
            roles: ['access'],
            traitLabels: [{ name: 'env', value: 'staging' }],
          }}
          editedRecurrence={{
            reviewDayOfMonth: reviewDayOfMonthOpts[0],
            reviewFrequency: reviewFrequencyOpts[1],
          }}
          setEditedRecurrence={() => null}
          reviewNotes="Removed expired members"
          setReviewNotes={() => null}
          originalMembers={mockMembers}
          editedMembers={mockMembers.slice(2)}
        />
      </Validation>
    </PreviewWrapper>
  );
};

const PreviewWrapper = styled(Box)`
  border-radius: ${p => p.theme.radii[3]}px;
  box-shadow: ${p => p.theme.boxShadow[1]};
  transform: var(--feature-preview-scale);
  background-color: ${p => p.theme.colors.levels.surface};
`;
