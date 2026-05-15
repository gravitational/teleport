import { Link as InternalLink } from 'react-router';
import styled from 'styled-components';

import { Box, H1, H2, Link, Stack, Text } from 'design';
import { Terminal } from 'design/Icon';

import cfg from 'e-teleport/config';
import { FeatureBox } from 'teleport/components/Layout/Layout';
import { useNoMinWidth } from 'teleport/Main';

import { BeamsCard } from './components';

const PAGE_MAX_WIDTH = 800;

export function BeamsList() {
  useNoMinWidth();

  return (
    <FeatureBox>
      <Box maxWidth={PAGE_MAX_WIDTH} mx="auto" width="100%">
        <H1 mt={5} mb={5}>
          My Beams
        </H1>
        <BeamsCard>
          <Stack alignItems="center" gap={3}>
            <Terminal size="extra-large" />
            <H2 textAlign="center">
              Viewing your Beams in-browser is coming soon.
            </H2>
            <Text typography="body1" textAlign="center">
              In the meantime, visit the{' '}
              <BoldLink as={InternalLink} to={cfg.getBeamsQuickstartRoute()}>
                Beams Quickstart
              </BoldLink>{' '}
              to create your first beam.
            </Text>
            <Text typography="body1" textAlign="center">
              Once you&apos;re in the terminal, view your Beams any time by
              running <Command>tsh beams ls</Command>.
            </Text>
          </Stack>
        </BeamsCard>
      </Box>
    </FeatureBox>
  );
}

const BoldLink = styled(Link)`
  font-weight: ${({ theme }) => theme.fontWeights.bold};
  text-decoration: none;
`;

const Command = styled.code`
  font-family: ${({ theme }) => theme.fonts.mono};
  color: ${({ theme }) => theme.colors.interactive.solid.accent.default};
`;
