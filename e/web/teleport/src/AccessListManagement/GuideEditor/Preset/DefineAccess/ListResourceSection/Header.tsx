import { Box, Link as ExternalLink, Flex, Text } from 'design';
import { Mark, MarkInverse } from 'design/Mark';
import { H2, H3 } from 'design/Text';
import { IconTooltip } from 'design/Tooltip';

import { StyledUl } from 'e-teleport/AccessListManagement/GuideEditor/Shared';

import { ListResourceAccessFields } from '../../role/listaccess';
import { getResourceKindName, getResourceRbacLink } from '../shared';

/**
 * Header for different resource access types.
 */
export function Header({
  selectedResourceTab,
}: {
  selectedResourceTab: ListResourceAccessFields;
}) {
  switch (selectedResourceTab) {
    case 'github_permissions':
      return (
        <>
          <H2 mt={2}>Define Git Server Access</H2>
          <Text mt={2}>Select which GitHub organizations is allowed:</Text>
        </>
      );

    case 'app_labels':
    case 'db_labels':
    case 'kubernetes_labels':
    case 'node_labels':
    case 'windows_desktop_labels':
    case 'linux_desktop_labels':
      const { resourceKind, byline } = getResourceKindName(selectedResourceTab);
      return (
        <>
          <Flex alignItems={'center'} gap={2} mt={2}>
            <H2 css={{ textTransform: 'capitalize' }}>
              Define {resourceKind} Access
            </H2>
            <IconTooltip sticky>
              <Text>
                Learn how{' '}
                <ExternalLink
                  target="_blank"
                  href={getResourceRbacLink(selectedResourceTab)}
                >
                  access to {byline}
                </ExternalLink>{' '}
                works.
              </Text>
            </IconTooltip>
          </Flex>

          <Box mt={2}>
            <Text>
              Click on labels or type the labels using <Mark>key: value</Mark>{' '}
              syntax, for example <Mark>environment: staging</Mark>.
            </Text>
            <Flex alignItems="center" gap={1}>
              <Text>
                You can also use special value <Mark>*</Mark> to allow all{' '}
                {byline}.
              </Text>
              <IconTooltip>
                <H3>Wildcard Options</H3>
                <StyledUl>
                  <li>
                    Type <MarkInverse>*: *</MarkInverse> to allow access to all{' '}
                    {byline} regardless of labels.
                  </li>
                  <li>
                    Type <MarkInverse>&lt;YOUR_KEY&gt;: *</MarkInverse> that
                    matches {byline} with label{' '}
                    <MarkInverse>YOUR_KEY</MarkInverse> with any value.
                  </li>
                </StyledUl>
              </IconTooltip>
            </Flex>
          </Box>
        </>
      );

    default:
      selectedResourceTab satisfies never;
  }
}
