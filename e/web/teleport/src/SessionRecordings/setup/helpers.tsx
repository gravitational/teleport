import { Database, Kubernetes, Server } from 'design/Icon';

import { type ResourceKind } from './schema/types';

export function resourceTypeIcon(type: ResourceKind) {
  switch (type) {
    case 'db':
      return <Database size="small" />;
    case 'k8s':
      return <Kubernetes size="small" />;
    case 'ssh':
      return <Server size="small" />;
  }
}

export function resourceTypeToLabel(type: ResourceKind) {
  switch (type) {
    case 'db':
      return 'Database';
    case 'k8s':
      return 'Kubernetes';
    case 'ssh':
      return 'SSH';
    default:
      return type;
  }
}
