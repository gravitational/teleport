import {
  Application,
  AuthSettings,
  Cluster,
  CreateGatewayParams,
  Database,
  Gateway,
  Kube,
  LoginLocalParams,
  LoginPasswordlessParams,
  LoginSsoParams,
  Server,
  TshAbortController,
  TshAbortSignal,
  TshClient,
} from '../types';

export class MockTshClient implements TshClient {
  listRootClusters: () => Promise<Cluster[]>;
  listLeafClusters: (clusterUri: string) => Promise<Cluster[]>;
  listApps: (clusterUri: string) => Promise<Application[]>;
  listKubes: (clusterUri: string) => Promise<Kube[]>;
  listDatabases: (clusterUri: string) => Promise<Database[]>;
  listDatabaseUsers: (dbUri: string) => Promise<string[]>;
  listServers: (clusterUri: string) => Promise<Server[]>;
  createAbortController: () => TshAbortController;
  addRootCluster: (addr: string) => Promise<Cluster>;

  listGateways: () => Promise<Gateway[]>;
  createGateway: (params: CreateGatewayParams) => Promise<Gateway>;
  removeGateway: (gatewayUri: string) => Promise<undefined>;
  restartGateway: (gatewayUri: string) => Promise<undefined>;
  setGatewayTargetSubresourceName: (
    gatewayUri: string,
    targetSubresourceName: string
  ) => Promise<Gateway>;
  setGatewayLocalPort: (
    gatewayUri: string,
    localPort: string
  ) => Promise<Gateway>;

  getCluster: (clusterUri: string) => Promise<Cluster>;
  getAuthSettings: (clusterUri: string) => Promise<AuthSettings>;
  removeCluster: (clusterUri: string) => Promise<undefined>;
  loginLocal: (
    params: LoginLocalParams,
    abortSignal?: TshAbortSignal
  ) => Promise<undefined>;
  loginSso: (
    params: LoginSsoParams,
    abortSignal?: TshAbortSignal
  ) => Promise<undefined>;
  loginPasswordless: (
    params: LoginPasswordlessParams,
    abortSignal?: TshAbortSignal
  ) => Promise<undefined>;
  logout: (clusterUri: string) => Promise<undefined>;
}
