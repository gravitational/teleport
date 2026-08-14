import styled from 'styled-components';

import { Box } from 'design';

import { AccessCard } from '../AccessCard';
import { mockAccessLists } from './fixtures';
import { Description, Feature, FeatureProps, Title } from './Shared';

export const GetVisibility = ({ active, onClick, isSliding }: FeatureProps) => {
  return (
    <Feature $active={active} onClick={onClick} $isSliding={isSliding}>
      <Title>Get visibility into access</Title>
      <Description>
        <span css={{ display: 'var(--feature-text-display)' }}>
          See why access has been granted and who has granted it.{' '}
        </span>
        See people responsible for access.
      </Description>
    </Feature>
  );
};

export const GetVisibilityPreview = () => {
  return (
    <PreviewWrapper>
      {mockAccessLists.map(accessList => (
        <AccessCard
          key={accessList.id}
          onlyRender={true}
          accessList={accessList}
          onClick={() => null}
        />
      ))}
    </PreviewWrapper>
  );
};

const PreviewWrapper = styled(Box)`
  display: grid;
  grid-template-columns: repeat(
    2,
    minmax(350px, 1fr)
  ); /* Two equal-width columns */
  gap: 16px; /* Spacing between grid items */
  transform: var(--feature-preview-scale);
  max-width: 700px;
  @media (max-width: 1445px) {
    margin-top: -70px;
  }
`;
