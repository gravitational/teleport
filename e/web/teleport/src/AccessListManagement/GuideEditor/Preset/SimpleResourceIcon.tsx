import {
  AmazonAws,
  Application,
  Database,
  Git,
  Kubernetes,
  Linux,
  Server,
  Windows,
} from 'design/Icon';
import { IconSize } from 'design/Icon/Icon';

import { DefinableResourceAccessFields } from './role/listaccess';

export function SimpleResourceIcon({
  field,
  size,
}: {
  field: DefinableResourceAccessFields;
  size?: IconSize;
}) {
  switch (field) {
    case 'app_labels':
      return <Application size={size} />;
    case 'awsIc':
      return <AmazonAws size={size} />;
    case 'db_labels':
      return <Database size={size} />;
    case 'github_permissions':
      return <Git size={size} />;
    case 'kubernetes_labels':
      return <Kubernetes size={size} />;
    case 'node_labels':
      return <Server size={size} />;
    case 'windows_desktop_labels':
      return <Windows size={size} />;
    case 'linux_desktop_labels':
      return <Linux size={size} />;
  }
}
