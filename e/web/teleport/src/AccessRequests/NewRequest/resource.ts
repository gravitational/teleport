import { ResourceIdKind } from 'teleport/services/agents';

/** Available request kinds for resource-based and role-based access requests. */
export type ResourceKind = ResourceIdKind | 'role' | 'resource';

export type ResourceMap = {
  [K in ResourceIdKind | 'role']: Record<string, string>;
};

export function getEmptyResourceState(): ResourceMap {
  return {
    node: {},
    db: {},
    app: {},
    kube_cluster: {},
    user_group: {},
    windows_desktop: {},
    role: {},
  };
}
