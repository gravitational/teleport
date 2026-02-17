import { Link } from 'react-router-dom';
import styled from 'styled-components';

import { Flex } from 'design';
import { ChevronLeft } from 'design/Icon';

import cfg from 'teleport/config';
import useStickyClusterId from 'teleport/useStickyClusterId';

export function IntegrationSessionSummariesHeader() {
  const { clusterId } = useStickyClusterId();

  return (
    <Flex
      alignItems="center"
      borderBottom={1}
      borderColor="interactive.tonal.neutral.0"
      width={'100%'}
      pl={6}
      py={1}
      gap={1}
      data-testid="aws-oidc-header"
    >
      <BackLink to={cfg.getRecordingsRoute(clusterId)}>
        <ChevronLeft size="small" />
        Back to Session Recordings
      </BackLink>
    </Flex>
  );
}

const BackLink = styled(Link)`
  color: ${p => p.theme.colors.text.slightlyMuted};
  text-decoration: none;
  font-weight: 500;
  display: flex;
  align-items: center;
  gap: ${p => p.theme.space[2]}px;
`;
