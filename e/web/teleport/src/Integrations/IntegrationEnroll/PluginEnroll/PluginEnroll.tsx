import { useParams } from 'react-router';
import { NotFound } from 'design/CardError';
import { Plugin, PluginKind } from 'teleport/services/integrations';

import { pluginMap } from './plugins';
import { PluginEnrollOAuth } from './PluginEnrollOauth';
import { PluginEnrollSingleStep } from './PluginEnrollSingleStep';
import { PluginProvider } from './MultiStep/usePlugin';
import { PluginEnrollMultiStep } from './PluginEnrollMultiStep';

export type OAuthPluginRegistered = {
  name: string;
  eventId: string;
  slack?: { fallback_channel: string };
};

type OAuthPluginResponse = {
  kind: 'oauth';
  error?: string | null;
  errorDescription: string | null;
  success?: OAuthPluginRegistered;
};

type StaticPluginResponse = {
  kind: 'static';
  plugin?: Plugin;
};

export type PluginEnrollResponse = StaticPluginResponse | OAuthPluginResponse;

export function PluginEnroll() {
  const { type: selectedPluginType } = useParams<{ type: PluginKind }>();

  const plugin = pluginMap[selectedPluginType];
  if (!plugin || !plugin.cloudHostable) {
    return (
      <NotFound
        message={`plugin type ${selectedPluginType} is either not \
        defined in pluginMap or not hostable`}
      />
    );
  }

  if (plugin.isOAuth) {
    return <PluginEnrollOAuth plugin={plugin} />;
  }

  if (!plugin.views || plugin.views().length === 0) {
    return <PluginEnrollSingleStep plugin={plugin} />;
  }

  // MultiStep:
  return (
    <PluginProvider selectedPlugin={plugin}>
      <PluginEnrollMultiStep />
    </PluginProvider>
  );
}
