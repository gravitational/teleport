import { JSX } from 'react';
import { Link as InternalLink } from 'react-router-dom';

import { Box, Link as ExternalLink, Flex, Text } from 'design';
import { IconTooltip } from 'design/Tooltip';
import { StyledUl } from 'shared/components/UnifiedResources/shared/StatusInfo';

import cfg from 'teleport/config';
import useTeleport from 'teleport/useTeleport';

import { getResourceKindName, getResourceRbacLink } from './shared';

export function EmptyList({ resourceField }: { resourceField: 'awsIc' }) {
  const teleCtx = useTeleport();
  const { resourceKind, byline } = getResourceKindName(resourceField);

  let noAccess: JSX.Element;
  let cta: JSX.Element;

  switch (resourceField) {
    case 'awsIc':
      noAccess = <Text>or {resourceKind} is not integrated.</Text>;
      cta = (
        <li>
          Or{' '}
          <InternalLink
            to={cfg.getIntegrationEnrollRoute('aws-identity-center')}
            target="_blank"
          >
            integrate
          </InternalLink>{' '}
          with AWS Identity Center
        </li>
      );
      break;

    default:
      resourceField satisfies never;
  }
  return (
    <Flex
      p={8}
      pt={5}
      width="100%"
      mx="auto"
      alignItems="center"
      justifyContent="center"
    >
      <Box textAlign="center">
        <Text bold css={{ textTransform: 'capitalize' }}>
          No {resourceKind} Found
        </Text>
        <Text>You may not have permission to access any {byline},</Text>
        <Flex alignItems={'center'} justifyContent={'center'} gap={2}>
          {noAccess}
          <IconTooltip sticky>
            <StyledUl>
              <li>
                Check your{' '}
                <InternalLink
                  target="_blank"
                  css={`
                    color: ${p => p.theme.colors.tooltip.inverseLinkDefault};
                  `}
                  to={`${cfg.routes.users}?user=${encodeURIComponent(teleCtx.storeUser.getUsername())}`}
                >
                  roles
                </InternalLink>{' '}
                grants{' '}
                <ExternalLink
                  target="_blank"
                  href={getResourceRbacLink(resourceField)}
                  css={`
                    color: ${p => p.theme.colors.tooltip.inverseLinkDefault};
                  `}
                >
                  access
                </ExternalLink>{' '}
                to {byline}
              </li>
              {cta}
            </StyledUl>
          </IconTooltip>
        </Flex>
      </Box>
    </Flex>
  );
}
