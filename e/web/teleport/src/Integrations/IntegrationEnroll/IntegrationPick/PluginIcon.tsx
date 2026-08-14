import { SpaceProps } from 'design/system';

import { IntegrationIcon } from 'teleport/Integrations/Enroll';

import { pluginMap } from '../PluginEnroll/plugins';

export function PluginIcon({ size, type, ...props }: Props) {
  const name = pluginMap[type]?.icon;
  if (!name) {
    return null;
  }
  return <IntegrationIcon {...props} size={size} name={name} />;
}

interface Props extends SpaceProps {
  size?: number;
  type: string;
}
