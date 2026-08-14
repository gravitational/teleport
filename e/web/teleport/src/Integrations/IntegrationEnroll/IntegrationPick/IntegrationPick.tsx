import { useEffect, useState } from 'react';

import { Alert } from 'design';
import useAttempt from 'shared/hooks/useAttemptNext';

import {
  type CloudHostablePlugin,
  type SelfHostedPlugin,
} from 'e-teleport/services/plugins';
import useTeleport from 'e-teleport/useTeleportE';
import {
  integrations as botIntegrations,
  BotTile,
  type BotIntegration,
} from 'teleport/Bots/Add/AddBotsPicker';
import cfg from 'teleport/config';
import {
  installableIntegrations,
  type IntegrationTileSpec as GenericIntegration,
} from 'teleport/Integrations/Enroll/IntegrationTiles/integrations';
import {
  IntegrationTileWithSpec,
  IntegrationPicker as SharedIntegrationPicker,
} from 'teleport/Integrations/Enroll/Shared';
import { sortByDisplayName } from 'teleport/Integrations/Enroll/Shared/IntegrationPicker';
import { useNoMinWidth } from 'teleport/Main';
import { PluginKind } from 'teleport/services/integrations';

import { plugins as defaultPlugins } from '../PluginEnroll/plugins';
import { integrationsE } from './integrations';
import { PluginTile } from './PluginTile';

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

type Integration =
  | GenericIntegration
  | BotIntegration
  | CloudHostablePlugin
  | SelfHostedPlugin;

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

export function IntegrationPick() {
  const ctx = useTeleport();
  const hasPluginAccess = ctx.storeUser.getPluginsAccess().create;
  const hasIntegrationAccess = ctx.storeUser.getIntegrationsAccess().create;
  const hasCreateBotPermission = ctx.getFeatureFlags().addBots;
  const canCreate = [
    {
      value: ctx.storeUser.getPluginsAccess().create,
      label: 'plugin.create',
    },
    {
      value: ctx.storeUser.getIntegrationsAccess().create,
      label: 'integration.create',
    },
  ].some(perm => perm.value);

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

  useNoMinWidth();
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
  }, []);

  const isGuided = (i: Integration) => {
    if (i.type == 'bot') {
      return i.guided;
    }

    if (i.type === 'integration') {
      return true;
    }

    if (
      // already enrolled, Okta integration may be partially enrolled;
      // we want to allow continuing the enrollment process
      (plugins.enrolled.includes(i.type) && i.type !== 'okta') ||
      // self-hosted plugins not guided
      (isNotCloudHostablePlugin(i) && plugins.selfHosted.includes(i))
    ) {
      return false;
    }

    return true;
  };

  const initialSort = (a: Integration, b: Integration) => {
    return (
      (isGuided(b) ? (isGuided(a) ? 0 : 1) : isGuided(a) ? -1 : 0) ||
      sortByDisplayName(a, b)
    );
  };

  const integrations = [
    ...plugins.available,
    ...plugins.selfHosted,
    ...installableIntegrations(),
    ...integrationsE,
    ...botIntegrations,
  ];

  const renderIntegration = (i: Integration) => {
    if (i.type === 'integration') {
      return (
        <IntegrationTileWithSpec
          key={i.kind}
          spec={i}
          hasIntegrationAccess={hasIntegrationAccess}
          hasExternalAuditStorage={hasExternalAuditStorageAccess}
        />
      );
    }

    if (i.type === 'bot') {
      return (
        <BotTile
          key={i.kind}
          integration={i}
          hasCreateBotPermission={hasCreateBotPermission}
        />
      );
    }

    return (
      <PluginTile
        pluginAlreadyEnrolled={
          // Okta integration may be partially enrolled; we want to allow continuing the enrollment process
          plugins.enrolled.includes(i.type) && i.type !== 'okta'
        }
        key={i.type}
        plugin={i}
        hasAccess={hasPluginAccess}
      />
    );
  };

  const isLoading = attempt.status === 'processing';
  const isFailed = attempt.status === 'failed';

  return (
    <SharedIntegrationPicker
      integrations={integrations}
      renderIntegration={renderIntegration}
      initialSort={initialSort}
      canCreate={canCreate}
      isLoading={isLoading}
      ErrorMessage={isFailed && <Alert>{attempt.statusText}</Alert>}
    />
  );
}
