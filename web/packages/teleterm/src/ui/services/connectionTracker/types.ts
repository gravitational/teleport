import { DatabaseUri, GatewayUri, KubeUri, ServerUri } from 'teleterm/ui/uri';

type TrackedConnectionBase = {
  connected: boolean;
  id: string;
  title: string;
};

export interface TrackedServerConnection extends TrackedConnectionBase {
  kind: 'connection.server';
  title: string;
  serverUri: ServerUri;
  login: string;
}

export interface TrackedGatewayConnection extends TrackedConnectionBase {
  kind: 'connection.gateway';
  targetUri: DatabaseUri;
  targetName: string;
  targetUser?: string;
  port?: string;
  gatewayUri: GatewayUri;
  targetSubresourceName?: string;
}

export interface TrackedKubeConnection extends TrackedConnectionBase {
  kind: 'connection.kube';
  kubeConfigRelativePath: string;
  kubeUri: KubeUri;
}

export type TrackedConnection =
  | TrackedServerConnection
  | TrackedGatewayConnection
  | TrackedKubeConnection;

export type ExtendedTrackedConnection = TrackedConnection & {
  clusterName: string;
};
