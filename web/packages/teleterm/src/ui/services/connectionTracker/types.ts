type TrackedConnectionBase = {
  connected: boolean;
  id: string;
  title: string;
};

export interface TrackedServerConnection extends TrackedConnectionBase {
  kind: 'connection.server';
  title: string;
  serverUri: string;
  login: string;
}

export interface TrackedGatewayConnection extends TrackedConnectionBase {
  kind: 'connection.gateway';
  targetUri: string;
  targetName: string;
  targetUser?: string;
  port?: string;
  gatewayUri: string;
  targetSubresourceName?: string;
}

export interface TrackedKubeConnection extends TrackedConnectionBase {
  kind: 'connection.kube';
  kubeConfigRelativePath: string;
  kubeUri: string;
}

export type TrackedConnection =
  | TrackedServerConnection
  | TrackedGatewayConnection
  | TrackedKubeConnection;

export type ExtendedTrackedConnection = TrackedConnection & {
  clusterName: string;
};
