import { Link as InternalLink } from 'react-router';

import { ButtonText, Flex, Text } from 'design';
import { Plugs } from 'design/Icon';
import { HoverTooltip } from 'design/Tooltip';

import cfg from 'teleport/config';

export function IntegrationSessionSummariesHeader() {
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
      <HoverTooltip placement="bottom" tipContent="Back to Integrations">
        <ButtonText
          size="small"
          as={InternalLink}
          to={cfg.routes.integrations}
          aria-label="integrations-table"
          color="text.slightlyMuted"
        >
          <Plugs size="small" />
        </ButtonText>
      </HoverTooltip>

      <Text typography="body3" color="text.slightlyMuted">
        {'/'} Session Summaries
      </Text>
    </Flex>
  );
}
