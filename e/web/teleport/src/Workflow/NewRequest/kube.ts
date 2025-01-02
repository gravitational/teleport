import { generatePath, matchPath } from 'react-router';

import { RequestableResourceKind } from 'shared/components/AccessRequests/NewRequest';

/**
 * Modeled after how backend creates access request resource IDs:
 *
 * :teleportClusterName/:resourceKind/:resourceName/:subResourceName
 *
 * `resourceKind` should equal/refer to backend KindXXX consts:
 * https://github.com/gravitational/teleport/blob/22edf09261f6c78ed6dd59b9202e4b2fb0cf452c/api/types/constants.go
 *
 * examples of how some resource IDs are constructed:
 * https://github.com/gravitational/teleport/blob/3e75921c8a6e146f7962b28fcfde575e7ca7d8a4/api/types/resource_ids_test.go#L25
 *
 */
const accessRequestResourceIdPath =
  ':teleportClusterName/:resourceKind/:resourceName/:subResourceName';

export type AccessRequestResourceIdParam = {
  /**
   * Name of teleport cluster name that this resource belongs to.
   */
  teleportClusterName: string;
  resourceKind: RequestableResourceKind;
  resourceName: string;
  /**
   * Optional field but marked required by "react-router generatePath"
   * type requirement.
   *
   * Some resources have sub resources eg: resource kube_cluster has
   * sub resources like "namespace, secrets, pods, etc".
   *
   * Requesting sub resource example (kube namespace):
   *   - resourceKind: "namespace"
   *   - resourceName: "the name of kube cluster that contains the namespace"
   *   - subResourceName: "the name of the namespace"
   */
  subResourceName: string;
};

/**
 * Returns a unique ID for an access request resource.
 */
export function getResourceIdUri(params: AccessRequestResourceIdParam) {
  return generatePath(accessRequestResourceIdPath, params);
}

export function parseResourceIdUri(uri: string) {
  return matchPath<AccessRequestResourceIdParam>(
    uri,
    accessRequestResourceIdPath
  );
}
