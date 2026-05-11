import { Box, Text } from 'design';
import { FeatureName } from 'design/constants';

import {
  getCTAForPlugin,
  pluginTypeToIntegrationEnrollKind,
  type CloudHostablePlugin,
  type SelfHostedPlugin,
} from 'e-teleport/services/plugins';
import { ButtonLockedFeature } from 'teleport/components/ButtonLockedFeature';
import {
  BadgeTitle,
  ToolTipNoPermBadge,
} from 'teleport/components/ToolTipNoPermBadge';
import cfg from 'teleport/config';
import { Tile } from 'teleport/Integrations/Enroll/Shared';
import { PluginKind } from 'teleport/services/integrations';
import {
  IntegrationEnrollEvent,
  userEventService,
} from 'teleport/services/userEvent';

import { pluginMap } from '../PluginEnroll/plugins';

export function PluginTile({
  pluginAlreadyEnrolled = false,
  plugin,
  hasAccess,
}: {
  pluginAlreadyEnrolled?: boolean;
  plugin: CloudHostablePlugin | SelfHostedPlugin;
  hasAccess: boolean;
}) {
  const hostedButNoAccess = !hasAccess && plugin.cloudHostable;

  const pluginAccess: PluginAccess = (() => {
    if (plugin.requiresIgs && !cfg.entitlements.Identity.enabled) {
      return 'requires-identity';
    }
    return hostedButNoAccess ? 'denied' : 'allowed';
  })();

  const pluginEnrollable = pluginAccess === 'allowed' && !pluginAlreadyEnrolled;

  const tileDisabled = pluginAccess !== 'allowed';

  const pluginIcon = pluginMap[plugin.type]?.icon;

  function getPluginLink() {
    if (plugin.cloudHostable) {
      const url =
        pluginEnrollable && !tileDisabled
          ? cfg.getIntegrationEnrollRoute(plugin.type)
          : undefined;

      return {
        external: false,
        url: url,
      };
    }

    if (tileDisabled || (!pluginEnrollable && plugin.cloudHostable)) {
      return {
        external: false,
        url: undefined,
      };
    }

    if (!plugin.cloudHostable) {
      return {
        external: true,
        url: plugin.url,
        onClick: !tileDisabled
          ? () => {
              userEventService.captureIntegrationEnrollEvent({
                event: IntegrationEnrollEvent.Started,
                eventData: {
                  id: crypto.randomUUID(),
                  kind: pluginTypeToIntegrationEnrollKind(plugin.type),
                },
              });
            }
          : undefined,
      };
    }

    return undefined;
  }

  const Badge = tileDisabled ? (
    <NoAccessTooltip
      pluginAccess={pluginAccess}
      pluginName={plugin.name}
      pluginType={plugin.type}
    />
  ) : undefined;

  return (
    <Tile
      title={plugin.name}
      description={plugin.description}
      hasAccess={pluginAccess === 'allowed'}
      data-testid={`tile-${plugin.type}`}
      enrolled={pluginAlreadyEnrolled}
      link={getPluginLink()}
      icon={pluginIcon}
      tags={plugin.tags}
      Badge={Badge}
    />
  );
}

type PluginAccess = 'allowed' | 'denied' | 'requires-identity';

function NoAccessTooltip({
  pluginAccess,
  pluginName,
  pluginType,
}: {
  pluginAccess: PluginAccess;
  pluginName: string;
  pluginType: PluginKind;
}) {
  switch (pluginAccess) {
    case 'denied':
      return (
        <ToolTipNoPermBadge badgeTitle={BadgeTitle.LackingPermissions}>
          <Box>
            <Text>
              You are not able to add this plugin. There are two possible
              reasons for this:
            </Text>
            <ul style={{ paddingLeft: 16, marginBottom: 2, marginTop: 2 }}>
              <li>
                Your cluster is not configured to support this plugin or hosted
                plugins is not enabled.
              </li>
              <li>
                You don’t have sufficient permissions to create a plugin. Reach
                out to your Teleport administrator to request additional
                permissions.
              </li>
            </ul>
          </Box>
        </ToolTipNoPermBadge>
      );
    case 'requires-identity':
      return (
        <ToolTipNoPermBadge badgeTitle={BadgeTitle.LackingIgs} sticky={true}>
          <Box textAlign="center" maxWidth="200px">
            <Text>
              Unlock {pluginName} plugin with {FeatureName.IdentityGovernance}
            </Text>
            <ButtonLockedFeature
              width="165px"
              mt={2}
              mb={1}
              noIcon
              event={getCTAForPlugin(pluginType)}
            >
              Contact Sales
            </ButtonLockedFeature>
          </Box>
        </ToolTipNoPermBadge>
      );
    default:
      return null;
  }
}
