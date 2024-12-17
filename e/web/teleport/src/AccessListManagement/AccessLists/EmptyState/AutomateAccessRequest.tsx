import styled from 'styled-components';

import {
  RequestorTimestamp,
  SuggestedAccessListTimestamp,
  Timeline,
  TimelineCommentAndReviewsContainer,
} from 'shared/components/AccessRequests/ReviewRequests';

import { Description, Feature, FeatureProps, Title } from './Shared';
import { mockAccessLists } from './fixtures';

export const AutomateAccessRequest = ({
  active,
  onClick,
  isSliding,
}: FeatureProps) => {
  return (
    <Feature $active={active} onClick={onClick} $isSliding={isSliding}>
      <Title>Automate access requests</Title>
      <Description>
        <span css={{ display: 'var(--feature-text-display)' }}>
          Replace ad hoc requests and reviews with self-service automation.{' '}
        </span>
        Auto-provision access for Okta groups, AWS, Databases, Kubernetes
        clusters and more.
      </Description>
    </Feature>
  );
};

export const AutomateAccessRequestPreview = () => {
  return (
    <AccessRequestPreviewWrapper>
      <Timeline css={{ height: '100%' }} />
      <RequestorTimestamp
        user={'user@example.com'}
        reason={'Requesting role "access" for node health checks and upgrades'}
        createdDuration={'7 minutes ago'}
        resources={[]}
      />
      <SuggestedAccessListTimestamp accessLists={mockAccessLists} />
    </AccessRequestPreviewWrapper>
  );
};

const AccessRequestPreviewWrapper = styled(TimelineCommentAndReviewsContainer)`
  border-radius: ${p => p.theme.radii[3]}px;
  box-shadow: ${p => p.theme.boxShadow[1]};
  transform: var(--feature-preview-scale);
`;
