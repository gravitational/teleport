/**
 * Copyright 2023 Gravitational, Inc.
 *
 * Licensed under the Apache License, Version 2.0 (the "License");
 * you may not use this file except in compliance with the License.
 * You may obtain a copy of the License at
 *
 *     http://www.apache.org/licenses/LICENSE-2.0
 *
 * Unless required by applicable law or agreed to in writing, software
 * distributed under the License is distributed on an "AS IS" BASIS,
 * WITHOUT WARRANTIES OR CONDITIONS OF ANY KIND, either express or implied.
 * See the License for the specific language governing permissions and
 * limitations under the License.
 */

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
import { ToolTipNoPermBadge } from 'teleport/components/ToolTipNoPermBadge';
import {
  IntegrationEnrollEvent,
  userEventService,
} from 'teleport/services/userEvent';
import { PluginKind } from 'teleport/services/integrations';
import cfg from 'teleport/config';

import { MachineIDIntegrationSection } from 'teleport/Integrations/Enroll/MachineIDIntegrationSection';

import useTeleport from 'e-teleport/useTeleportE';

import {
  HostedPlugin,
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
  available: HostedPlugin[];
  // selfHosted are plugins that doesn't have
  // onboarding support (link out to docs).
  selfHosted: SelfHostedPlugin[];
};

export function IntegrationPick() {
  const ctx = useTeleport();
  const hasPluginAccess = ctx.storeUser.getPluginsAccess().create;
  const hasIntegrationAccess = ctx.storeUser.getIntegrationsAccess().create;

  const { attempt, run } = useAttempt(
    hasPluginAccess && cfg.isCloud ? 'processing' : ''
  );
  const [plugins, setPlugins] = useState<Plugins>({
    // available describes plugins that teleport helps onboard and is
    // only supported in cloud (for the moment).
    //
    // So for enterprise, we will not render any tiles.
    //
    // For cloud, even if the user has no plugin access, we will still
    // render the available tiles but it will disabled.
    available: cfg.isCloud ? defaultPlugins.filter(isHostedPlugin) : [],
    // selfHosted tiles will be rendered to
    // both enterprise and cloud teleport.
    selfHosted: defaultPlugins.filter(isSelfHostedPlugin),
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

    if (hasPluginAccess && cfg.isCloud) {
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
          <Flex mb={2} gap={3}>
            <IntegrationTiles hasAccess={hasIntegrationAccess} />
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
            <Flex mb={2} gap={3}>
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
  type: HostedPlugin | SelfHostedPlugin;
  hasAccess: boolean;
}) {
  const isClickable = hasAccess && !pluginAlreadyEnrolled && plugin.hosted;

  let tileProps;

  if (isClickable) {
    tileProps = {
      as: InternalLink,
      to: cfg.getIntegrationEnrollRoute(plugin.type),
    };
  } else if (!plugin.hosted) {
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

  const hostedButNoAccess = !hasAccess && plugin.hosted;

  return (
    <IntegrationTile
      disabled={hostedButNoAccess}
      data-testid={`tile-${plugin.type}`}
      $exists={pluginAlreadyEnrolled}
      {...tileProps}
    >
      <PluginIcon my={3} type={plugin.type} />
      <Box mb={2} css={{ position: 'relative' }}>
        <Text>
          {plugin.name}
          {pluginAlreadyEnrolled && (
            <>
              {' '}
              <Icons.Check
                ml={1}
                data-testid="plugin-checkmark"
                color="success"
                css={`
                  position: absolute;
                  display: inline-flex;
                  align-items: center;
                  top: 0;
                  bottom: 0;
                `}
              />
            </>
          )}
        </Text>
      </Box>
      {hostedButNoAccess && (
        <ToolTipNoPermBadge
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
      )}
    </IntegrationTile>
  );
}

function isHostedPlugin(
  plugin: HostedPlugin | SelfHostedPlugin
): plugin is HostedPlugin {
  return plugin.hosted;
}

function isSelfHostedPlugin(
  plugin: HostedPlugin | SelfHostedPlugin
): plugin is SelfHostedPlugin {
  return !plugin.hosted;
}
