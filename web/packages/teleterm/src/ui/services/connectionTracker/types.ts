type TrackedConnectionBase = {
  kind: 'connection.server' | 'connection.gateway';
  connected: boolean;
};

export interface TrackedServerConnection extends TrackedConnectionBase {
  kind: 'connection.server';
  title: string;
  id: string;
  serverUri: string;
  login: string;
}

export interface TrackedGatewayConnection extends TrackedConnectionBase {
  kind: 'connection.gateway';
  title: string;
  id: string;
  targetUri: string;
  targetUser?: string;
  port?: string;
  gatewayUri: string;
}

export type TrackedConnection =
  | TrackedServerConnection
  | TrackedGatewayConnection;

export type ExtendedTrackedConnection = TrackedConnection & {
  clusterName: string;
};
