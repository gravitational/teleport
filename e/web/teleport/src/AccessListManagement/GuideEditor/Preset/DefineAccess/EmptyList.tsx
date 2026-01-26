import { JSX } from 'react';
import { Link as InternalLink } from 'react-router-dom';

import { Box, Link as ExternalLink, Flex, Text } from 'design';
import { IconTooltip } from 'design/Tooltip';

import cfg from 'teleport/config';
import useTeleport from 'teleport/useTeleport';

import { StyledUl } from '../../Shared';
import { DefinableResourceAccessFields } from '../role/listaccess';
import { getResourceKindName, getResourceRbacLink } from './shared';

/**
 * Used when initial querying of resources does not produce any result.
 * This can be for following reasons:
 * - cluster doesn't have any resources, adds CTA to enroll the resource
 * - user doesn't have the proper RBAC to list resources, solved by tooltip
 *   calling out to user to check their access
 */
export function EmptyList({
  resourceField,
}: {
  resourceField: DefinableResourceAccessFields;
}) {
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

    case 'github_permissions':
      noAccess = <Text>or {resourceKind} is not integrated.</Text>;
      cta = (
        <li>
          Or{' '}
          <ExternalLink
            to={cfg.getIntegrationEnrollRoute('aws-identity-center')}
            target="_blank"
          >
            integrate
          </ExternalLink>{' '}
          with AWS Identity Center
        </li>
      );
      break;
      break;
    case 'app_labels':
    case 'db_labels':
    case 'kubernetes_labels':
    case 'node_labels':
    case 'windows_desktop_labels':
      noAccess = <Text>or no {byline} are enrolled.</Text>;
      cta = (
        <li>
          Or{' '}
          <InternalLink
            // TODO(kimlisa): discover doesn't support query param,
            // which is needed to preserve state when going to a new tab.
            // Query param should support filtering by resource kinds.
            to={cfg.routes.discover}
            target="_blank"
          >
            enroll
          </InternalLink>{' '}
          your first {resourceKind}
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
                  to={`${cfg.routes.users}?user=${encodeURIComponent(teleCtx.storeUser.getUsername())}`}
                >
                  roles
                </InternalLink>{' '}
                grants{' '}
                <ExternalLink
                  target="_blank"
                  href={getResourceRbacLink(resourceField)}
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
