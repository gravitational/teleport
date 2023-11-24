import React, { useState, useEffect } from 'react';
import { Link as InternalLink } from 'react-router-dom';

import {
  Alert,
  Box,
  Flex,
  Link as ExternalLink,
  Text,
  Indicator,
} from 'design';
import * as Icons from 'design/Icon';
import useAttempt from 'shared/hooks/useAttemptNext';
import { FeatureHeader, FeatureHeaderTitle } from 'teleport/components/Layout';
import {
  IntegrationTile,
  IntegrationTiles,
  NoCodeIntegrationDescription,
} from 'teleport/Integrations/Enroll';
import {
  ToolTipNoPermBadge,
  BadgeTitle,
} from 'teleport/components/ToolTipNoPermBadge';
import { ButtonLockedFeature } from 'teleport/components/ButtonLockedFeature';
import {
  IntegrationEnrollEvent,
  userEventService,
} from 'teleport/services/userEvent';
import { PluginKind } from 'teleport/services/integrations';
import cfg from 'teleport/config';

import { MachineIDIntegrationSection } from 'teleport/Integrations/Enroll/MachineIDIntegrationSection';

import useTeleport from 'e-teleport/useTeleportE';

import { getCTAForPlugin } from 'e-teleport/services/plugins';

import {
  CloudHostablePlugin,
  plugins as defaultPlugins,
  SelfHostedPlugin,
  pluginTypeToIntegrationEnrollKind,
} from '../PluginEnroll/plugins';

import { PluginIcon } from './PluginIcon';

type Plugins = {
  // enrolled are names of plugin types that have
  // already been created.
  enrolled: PluginKind[];
  // available plugins are adapted for hosting,
  // and which the auth server is set up for,
  // and are plugins that are not already enrolled yet.
  available: CloudHostablePlugin[];
  // selfHosted are plugins that doesn't have
  // onboarding support (link out to docs).
  selfHosted: SelfHostedPlugin[];
};

export function IntegrationPick() {
  const ctx = useTeleport();
  const hasPluginAccess = ctx.storeUser.getPluginsAccess().create;
  const hasIntegrationAccess = ctx.storeUser.getIntegrationsAccess().create;
  const hasExternalAuditStorageAccess =
    ctx.storeUser.getExternalAuditStorageAccess().create;

  const { attempt, run } = useAttempt(hasPluginAccess ? 'processing' : '');
  const [plugins, setPlugins] = useState<Plugins>({
    // For enterprise, we will not render any tiles that are not 'selfHostable'.
    //
    // For cloud, even if the user has no plugin access, we will still
    // render the available tiles but it will be disabled.
    available: cfg.isCloud
      ? defaultPlugins.filter(isCloudHostablePlugin)
      : defaultPlugins.filter(isCloudHostablePlugin).filter(canBeSelfHosted),
    // selfHosted tiles will be rendered to
    // both enterprise and cloud teleport.
    selfHosted: defaultPlugins
      .filter(canBeSelfHosted)
      .filter(isNotCloudHostablePlugin),
    enrolled: [],
  });

  useEffect(() => {
    async function fetchAndMakePlugins() {
      const [supportedTypes, enrolledPlugins] = await Promise.all([
        ctx.pluginsService.fetchAvailableTypes(),
        ctx.pluginsService
          .fetchPlugins()
          .then(response => response.map(p => p.kind)),
      ]);

      setPlugins({
        enrolled: enrolledPlugins,
        selfHosted: plugins.selfHosted,
        available: plugins.available.filter(p =>
          supportedTypes.includes(p.type)
        ),
      });
    }

    if (hasPluginAccess) {
      run(() => fetchAndMakePlugins());
    }

    // Only requires fetching available/existing plugin types
    // once on init.
    // eslint-disable-next-line react-hooks/exhaustive-deps
  }, []);

  let content;
  if (attempt.status === 'processing') {
    content = (
      <Box textAlign="center" m={10}>
        <Indicator />
      </Box>
    );
  } else if (attempt.status === 'failed') {
    content = <Alert children={attempt.statusText} />;
  } else {
    content = (
      <Flex flexDirection="column" gap={4}>
        <Flex flexDirection="column">
          <NoCodeIntegrationDescription />
          <Flex mb={2} gap={3} flexWrap="wrap">
            <IntegrationTiles
              hasIntegrationAccess={hasIntegrationAccess}
              hasExternalAuditStorage={hasExternalAuditStorageAccess}
            />
            {plugins.available.map(p => (
              <PluginTile
                pluginAlreadyEnrolled={plugins.enrolled.includes(p.type)}
                key={p.type}
                type={p}
                hasAccess={hasPluginAccess}
              />
            ))}
          </Flex>
        </Flex>

        {plugins.selfHosted.length > 0 && (
          <Flex flexDirection="column">
            <Text fontWeight="bold" typography="h4">
              Self-Hosted Plugins
            </Text>
            <Text mb={3} typography="body1">
              There is a wide variety of plugins that you can source from the
              Teleport GitHub repository. See below for a sampling, or check out
              the base repo at{' '}
              <ExternalLink
                href="https://github.com/gravitational/teleport-plugins"
                target="_blank"
              >
                https://github.com/gravitational/teleport-plugins
              </ExternalLink>
              . Self-hosted plugins will not show up in your integration list,
              and must be managed outside of the Teleport UI.
            </Text>
            <Flex mb={2} gap={3} flexWrap="wrap">
              {plugins.selfHosted.map(p => (
                <PluginTile key={p.type} type={p} hasAccess={hasPluginAccess} />
              ))}
            </Flex>
          </Flex>
        )}

        <Flex flexDirection="column">
          <MachineIDIntegrationSection />
        </Flex>
      </Flex>
    );
  }

  return (
    <>
      <FeatureHeader>
        <FeatureHeaderTitle>Select Integration Type</FeatureHeaderTitle>
      </FeatureHeader>
      {content}
    </>
  );
}

function PluginTile({
  pluginAlreadyEnrolled = false,
  type: plugin,
  hasAccess,
}: {
  pluginAlreadyEnrolled?: boolean;
  type: CloudHostablePlugin | SelfHostedPlugin;
  hasAccess: boolean;
}) {
  const hostedButNoAccess = !hasAccess && plugin.cloudHostable;

  const pluginAccess: PluginAccess =
    plugin.disableForTeam && cfg.isUsageBasedBilling
      ? 'requires-enterprise'
      : hostedButNoAccess
      ? 'denied'
      : 'allowed';

  const pluginEnrollable = pluginAccess === 'allowed' && !pluginAlreadyEnrolled;

  let tileProps;

  if (pluginEnrollable && plugin.cloudHostable) {
    tileProps = {
      as: InternalLink,
      to: cfg.getIntegrationEnrollRoute(plugin.type),
    };
  } else if (!plugin.cloudHostable) {
    tileProps = {
      as: ExternalLink,
      href: plugin.url,
      target: '_blank',
      onClick: () => {
        userEventService.captureIntegrationEnrollEvent({
          event: IntegrationEnrollEvent.Started,
          eventData: {
            id: crypto.randomUUID(),
            kind: pluginTypeToIntegrationEnrollKind(plugin.type),
          },
        });
      },
    };
  }

  return (
    <IntegrationTile
      disabled={pluginAccess !== 'allowed'}
      data-testid={`tile-${plugin.type}`}
      $exists={pluginAlreadyEnrolled}
      {...tileProps}
    >
      <PluginIcon my={3} type={plugin.type} />
      <Box
        mb={2}
        style={{
          display: 'flex',
          flexDirection: 'row',
          justifyContent: 'center',
          alignItems: 'center',
        }}
      >
        <Text>{plugin.name}</Text>
        {pluginAlreadyEnrolled && (
          <Icons.Check
            ml={1}
            data-testid="plugin-checkmark"
            color="success"
            size="small"
          />
        )}
      </Box>
      <RenderTooltip
        pluginAccess={pluginAccess}
        pluginName={plugin.name}
        pluginType={plugin.type}
      />
    </IntegrationTile>
  );
}

function isCloudHostablePlugin(
  plugin: CloudHostablePlugin | SelfHostedPlugin
): plugin is CloudHostablePlugin {
  return plugin.cloudHostable;
}

function isNotCloudHostablePlugin(
  plugin: CloudHostablePlugin | SelfHostedPlugin
): plugin is Exclude<typeof plugin, CloudHostablePlugin> {
  return !plugin.cloudHostable;
}

function canBeSelfHosted(
  plugin: CloudHostablePlugin | SelfHostedPlugin
): boolean {
  return plugin.selfHostable;
}

type PluginAccess = 'allowed' | 'denied' | 'requires-enterprise';

function RenderTooltip({
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
        <ToolTipNoPermBadge
          badgeTitle={BadgeTitle.LackingPermissions}
          children={
            <Box>
              <Text>
                You are not able to add this plugin. There are two possible
                reasons for this:
              </Text>
              <ul style={{ paddingLeft: 16, marginBottom: 2, marginTop: 2 }}>
                <li>
                  Your cluster is not configured to support this plugin or
                  hosted plugins is not enabled.
                </li>
                <li>
                  You don’t have sufficient permissions to create a plugin.
                  Reach out to your Teleport administrator to request additional
                  permissions.
                </li>
              </ul>
            </Box>
          }
        />
      );
    case 'requires-enterprise':
      return (
        <ToolTipNoPermBadge
          badgeTitle={BadgeTitle.LackingEnterpriseLicense}
          sticky={true}
          children={
            <Box textAlign="center" maxWidth="200px">
              <Text>Unlock {pluginName} plugin with Teleport Enterprise</Text>
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
          }
        />
      );
    default:
      return null;
  }
}
