import { useParams } from 'react-router';

import { NotFound } from 'design/CardError';

import { PluginKind } from 'teleport/services/integrations';

import { PluginProvider } from './MultiStep/usePlugin';
import { PluginEnrollMultiStep } from './PluginEnrollMultiStep';
import { PluginEnrollSingleStep } from './PluginEnrollSingleStep';
import { pluginMap } from './plugins';

export function PluginEnroll() {
  const { type: selectedPluginType } = useParams<{ type: PluginKind }>();

  if (!selectedPluginType) {
    return <NotFound message="plugin type was not provided" />;
  }

  const plugin = pluginMap[selectedPluginType];
  if (!plugin || !plugin.cloudHostable) {
    return (
      <NotFound
        message={`plugin type ${selectedPluginType} is either not defined in pluginMap or not hostable`}
      />
    );
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
