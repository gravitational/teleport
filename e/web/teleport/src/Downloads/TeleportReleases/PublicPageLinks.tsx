import { ComponentType } from 'react';
import styled from 'styled-components';

import Box from 'design/Box';
import Flex from 'design/Flex';
import {
  ArrowSquareOut,
  Cluster,
  Laptop,
  ListMagnifyingGlass,
  Terminal,
} from 'design/Icon';
import { IconProps } from 'design/Icon/Icon';
import Text, { H2, Subtitle2 } from 'design/Text';

export const PublicPageLinks = ({ authVersion }: { authVersion: string }) => {
  return (
    <Box mb={3}>
      <H2 mb={3}>Download and deploy Teleport</H2>
      <Flex flexDirection="column" gap={3} width={'max-content'}>
        <DownloadOption
          icon={Laptop}
          title="Download client tools"
          description="Dev and resource access tools"
          url={`https://goteleport.com/download/client-tools?filter=ent&version=${authVersion}`}
        />
        <DownloadOption
          icon={Terminal}
          title="Deploy agents"
          description="Options for deploying Teleport Agents"
          url={`https://goteleport.com/download/deploy-teleport-agent?filter=ent&version=${authVersion}`}
        />
        <DownloadOption
          icon={Cluster}
          title="Deploy a Teleport cluster"
          description="Options for hosting a cluster"
          url={`https://goteleport.com/download/deploy-teleport-cluster?filter=ent&version=${authVersion}`}
        />
        <DownloadOption
          icon={ListMagnifyingGlass}
          title="See all downloads"
          description="Full list of Teleport's direct downloads"
          url={`https://goteleport.com/download/all-downloads?filter=ent&version=${authVersion}`}
        />
      </Flex>
    </Box>
  );
};

const DownloadOption = ({
  url,
  icon: Icon,
  title,
  description,
}: {
  url: string;
  icon: ComponentType<IconProps>;
  title: string;
  description: string;
}) => {
  return (
    <DownloadTile as="a" href={url} target="_blank" rel="noopener noreferrer">
      <Flex
        p={2}
        alignItems="center"
        justifyContent="center"
        borderRadius={3}
        bg="interactive.tonal.neutral.0"
        flexShrink={0}
        color="text.main"
      >
        <Icon />
      </Flex>
      <Flex flexDirection="column" gap={1} flex={1} alignSelf="stretch">
        <Text color="text.main" fontSize={3} fontWeight="bold">
          {title}
        </Text>
        <Subtitle2 color="text.slightlyMuted">{description}</Subtitle2>
      </Flex>
      <NewTabIcon />
    </DownloadTile>
  );
};

const DownloadTile = styled(Flex)`
  padding: ${({ theme }) => theme.space[4]}px;
  align-items: center;
  gap: ${({ theme }) => theme.space[4]}px;
  border-radius: ${({ theme }) => theme.radii[3]}px;
  border: 2px solid ${({ theme }) => theme.colors.interactive.tonal.neutral[0]};
  text-decoration: none;
  cursor: pointer;
  color: ${({ theme }) => theme.colors.text.main};

  &:hover {
    background: ${({ theme }) => theme.colors.levels.elevated};
    border-color: rgba(0, 0, 0, 0);
    box-shadow: ${({ theme }) => theme.boxShadow[3]};
  }
`;

const NewTabIcon = styled(ArrowSquareOut)`
  opacity: 0;
  flex-shrink: 0;
  color: ${({ theme }) => theme.colors.text.muted};

  ${DownloadTile}:hover & {
    opacity: 1;
  }
`;
