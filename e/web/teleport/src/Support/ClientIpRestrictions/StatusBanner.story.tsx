import Box from 'design/Box';
import Flex from 'design/Flex';
import { H3 } from 'design/Text';

import { CirUiState, StatusBanner } from './StatusBanner';

export default {
  title: 'TeleportE/ClientIpRestrictions/StatusBanner',
  component: StatusBanner,
};

// Ordered the way an allowlist moves through them, not alphabetically, so the
// copy can be read as the sequence a user actually sees.
const states: CirUiState[] = [
  'notConfigured',
  'draft',
  'pending',
  'active',
  'returningToDraft',
  'testRunApplying',
  'testRunActive',
  'testRunEnding',
  'expired',
  'unknown',
];

export const AllStates = () => (
  <Flex flexDirection="column" gap={4} maxWidth={720}>
    {states.map(state => (
      <Box key={state}>
        <H3 mb={2}>{state}</H3>
        <StatusBanner state={state} />
      </Box>
    ))}
  </Flex>
);

/**
 * The panel appends a live countdown to the title while a test run is on its
 * way out or under way; a static string stands in for it here.
 */
export const WithCountdown = () => (
  <Flex flexDirection="column" gap={4} maxWidth={720}>
    <Box>
      <H3 mb={2}>testRunApplying</H3>
      <StatusBanner state="testRunApplying" countdown="29:47" />
    </Box>
    <Box>
      <H3 mb={2}>testRunActive</H3>
      <StatusBanner state="testRunActive" countdown="12:03" />
    </Box>
  </Flex>
);
