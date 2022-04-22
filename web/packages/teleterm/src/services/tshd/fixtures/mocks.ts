import * as types from 'teleterm/services/tshd/types';

export class MockTshClient implements types.TshClient {
  listGateways: () => Promise<types.Gateway[]>;
  listRootClusters: () => Promise<types.Cluster[]>;
  listLeafClusters: (clusterUri: string) => Promise<types.Cluster[]>;
  listDatabases: (clusterUri: string) => Promise<types.Database[]>;
  listDatabaseUsers: (dbUri: string) => Promise<string[]>;
  listKubes: (clusterUri: string) => Promise<types.Kube[]>;
  listApps: (clusterUri: string) => Promise<types.Application[]>;
  listServers: (clusterUri: string) => Promise<types.Server[]>;
  addRootCluster: (clusterUri: string) => Promise<types.Cluster>;
  createGateway: (params: types.CreateGatewayParams) => Promise<types.Gateway>;
  createAbortController: () => types.TshAbortController;
  getCluster: (clusterUri: string) => Promise<types.Cluster>;
  getAuthSettings: (clusterUri: string) => Promise<types.AuthSettings>;
  ssoLogin: (clusterUri: string, pType: string, pName: string) => Promise<void>;
  removeGateway: (gatewayUri: string) => Promise<void>;
  login: (
    params: types.LoginParams,
    abortSignal?: types.TshAbortSignal
  ) => Promise<void>;
  logout: (clusterUri: string) => Promise<void>;
  removeCluster: (clusterUri: string) => Promise<void>;
}
