type NodeKind =
  | 'group'
  | 'resource'
  | 'user'
  | 'action'
  | 'request_access'
  | 'review_access';
type NodeSubKind =
  | 'user_group'
  | 'resource_group'
  | 'host'
  | 'request_access'
  | 'review_access';
export type QueryGraphResponse = {
  nodes: Node[];
  edges: Edge[];
};

type Edge = {
  from: string;
  to: string;
  type: string;
};

// Non-action node (e.g. a user, user group, resource, resource group).
type Node = {
  // id is a synthetic unique ID for frontend usage purposes
  id: string;
  // name is the resource name (e.g. username for user, group name for the group)
  name: string;
  kind: NodeKind;
  sub_kind: NodeSubKind;
  hostname?: string;
};
