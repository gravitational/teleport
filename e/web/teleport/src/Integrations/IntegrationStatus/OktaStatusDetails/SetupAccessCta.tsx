import { Link as InternalLink } from 'react-router';

import { Box, ButtonBorder, Flex, Text } from 'design';
import { MarkInverse } from 'design/Mark';
import { HoverTooltip } from 'design/Tooltip';

import { goToCreateAccessListFromOktaRoute } from 'e-teleport/AccessListManagement/CreateAccessList/route';
import cfg from 'teleport/config';

import { Panel, PanelTitle } from './Shared';

export function SetupAccessCta({
  enabledAppGroupSync,
  oktaOrgUrl,
}: {
  enabledAppGroupSync: boolean;
  oktaOrgUrl;
}) {
  return (
    <Panel>
      <Flex
        flexDirection="column"
        justifyContent="space-between"
        gap={2}
        height="100%"
      >
        <Box>
          <PanelTitle>Manage Access to Teleport Resources</PanelTitle>
          <Text pt={1}>
            Set up access to Teleport protected resources for your Okta user
            groups.
          </Text>
        </Box>
        <HoverTooltip
          tipContent={
            enabledAppGroupSync ? undefined : (
              <>
                Requires <MarkInverse>Apps and User Groups</MarkInverse> sync to
                be enabled.
              </>
            )
          }
        >
          <ButtonBorder
            size="large"
            width={!cfg.isCloud ? '100%' : '230px'}
            disabled={!enabledAppGroupSync}
            {...(enabledAppGroupSync && {
              as: InternalLink,
              ...goToCreateAccessListFromOktaRoute(oktaOrgUrl),
            })}
          >
            Set Up Access
          </ButtonBorder>
        </HoverTooltip>
      </Flex>
    </Panel>
  );
}
