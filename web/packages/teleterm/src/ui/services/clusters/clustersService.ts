import { useStore } from 'shared/libs/stores';

import isMatch from 'design/utils/match';
import { makeLabelTag } from 'teleport/components/formatters';
import { Label } from 'teleport/types';

import { routing } from 'teleterm/ui/uri';
import { NotificationsService } from 'teleterm/ui/services/notifications';
import { getClusterName } from 'teleterm/ui/utils/getClusterName';
import { Cluster } from 'teleterm/services/tshd/types';

import { ImmutableStore } from '../immutableStore';

import {
  AuthSettings,
  ClustersServiceState,
  CreateGatewayParams,
  LoginParams,
  SyncStatus,
  tsh,
} from './types';

export function createClusterServiceState(): ClustersServiceState {
  return {
    apps: new Map(),
    kubes: new Map(),
    clusters: new Map(),
    gateways: new Map(),
    servers: new Map(),
    dbs: new Map(),
    serversSyncStatus: new Map(),
    dbsSyncStatus: new Map(),
    kubesSyncStatus: new Map(),
    appsSyncStatus: new Map(),
  };
}

export class ClustersService extends ImmutableStore<ClustersServiceState> {
  state: ClustersServiceState = createClusterServiceState();

  constructor(
    public client: tsh.TshClient,
    private notificationsService: NotificationsService
  ) {
    super();
  }

  async addRootCluster(addr: string) {
    const cluster = await this.client.addRootCluster(addr);
    this.setState(draft => {
      draft.clusters.set(
        cluster.uri,
        this.removeInternalLoginsFromCluster(cluster)
      );
    });

    return cluster;
  }

  async logout(clusterUri: string) {
    await this.client.logout(clusterUri);
    await this.syncClusterInfo(clusterUri);
    this.removeResources(clusterUri);
  }

  async login(params: LoginParams, abortSignal: tsh.TshAbortSignal) {
    await this.client.login(params, abortSignal);
    await Promise.allSettled([
      this.syncRootClusterAndCatchErrors(params.clusterUri),
      // A temporary workaround until the gateways are able to refresh their own certs on incoming
      // connections.
      //
      // After logging in and obtaining fresh certs for the cluster, we need to make the gateways
      // obtain fresh certs as well. Currently, the only way to achieve that is to restart them.
      this.restartClusterGatewaysAndCatchErrors(params.clusterUri).then(() =>
        // Sync gateways to update their status, in case one of them failed to start back up.
        // In that case, that gateway won't be included in the gateway list in the tsh daemon.
        this.syncGateways()
      ),
    ]);
  }

  async restartClusterGatewaysAndCatchErrors(rootClusterUri: string) {
    await Promise.allSettled(
      this.findGateways(rootClusterUri).map(async gateway => {
        try {
          await this.restartGateway(gateway.uri);
        } catch (error) {
          const title = `Could not restart the database connection for ${gateway.targetUser}@${gateway.targetName}`;

          this.notificationsService.notifyError({
            title,
            description: error.message,
          });
        }
      })
    );
  }

  async syncRootClusterAndCatchErrors(clusterUri: string) {
    try {
      await this.syncRootCluster(clusterUri);
    } catch (e) {
      const cluster = this.findCluster(clusterUri);
      const clusterName =
        getClusterName(cluster) ||
        routing.parseClusterUri(clusterUri).params.rootClusterId;
      this.notificationsService.notifyError({
        title: `Could not synchronize cluster ${clusterName}`,
        description: e.message,
      });
    }
  }

  async syncRootCluster(clusterUri: string) {
    try {
      await Promise.all([
        // syncClusterInfo never fails with a retryable error, only syncLeafClusters does.
        this.syncClusterInfo(clusterUri),
        this.syncLeafClusters(clusterUri),
      ]);
    } finally {
      // Functions below handle their own errors, so we don't need to await them.
      //
      // Also, we wait for syncClusterInfo to finish first. When the response from it comes back and
      // it turns out that the cluster is not connected, these functions will immediately exit with
      // an error.
      //
      // Arguably, it is a bit of a race condition, as we assume that syncClusterInfo will return
      // before syncLeafClusters, but for now this is a condition we can live with.
      this.syncKubes(clusterUri);
      this.syncApps(clusterUri);
      this.syncDbs(clusterUri);
      this.syncServers(clusterUri);
      this.syncKubes(clusterUri);
      this.syncGateways();
    }
  }

  async syncLeafCluster(clusterUri: string) {
    try {
      // Sync leaf clusters list, so that in case of an error that can be resolved with login we can
      // propagate that error up.
      const rootClusterUri = routing.ensureRootClusterUri(clusterUri);
      await this.syncLeafClustersList(rootClusterUri);
    } finally {
      this.syncLeafClusterResourcesAndCatchErrors(clusterUri);
    }
  }

  private async syncLeafClusterResourcesAndCatchErrors(clusterUri: string) {
    // Functions below handle their own errors, so we don't need to await them.
    this.syncKubes(clusterUri);
    this.syncApps(clusterUri);
    this.syncDbs(clusterUri);
    this.syncServers(clusterUri);
    this.syncKubes(clusterUri);
    this.syncGateways();
  }

  async syncRootClusters() {
    try {
      const clusters = await this.client.listRootClusters();
      this.setState(draft => {
        draft.clusters = new Map(clusters.map(c => [c.uri, c]));
      });
      clusters
        .filter(c => c.connected)
        .forEach(c => this.syncRootClusterAndCatchErrors(c.uri));
    } catch (error) {
      this.notificationsService.notifyError({
        title: 'Could not fetch root clusters',
        description: error.message,
      });
    }
  }

  async syncCluster(clusterUri: string) {
    const cluster = this.findCluster(clusterUri);
    if (!cluster) {
      throw Error(`missing cluster: ${clusterUri}`);
    }

    if (cluster.leaf) {
      return await this.syncLeafCluster(clusterUri);
    } else {
      return await this.syncRootCluster(clusterUri);
    }
  }

  async syncGateways() {
    try {
      const gws = await this.client.listGateways();
      this.setState(draft => {
        draft.gateways = new Map(gws.map(g => [g.uri, g]));
      });
    } catch (error) {
      this.notificationsService.notifyError({
        title: 'Could not synchronize database connections',
        description: error.message,
      });
    }
  }

  async syncKubes(clusterUri: string) {
    const cluster = this.state.clusters.get(clusterUri);
    if (!cluster.connected) {
      this.setState(draft => {
        draft.kubesSyncStatus.delete(clusterUri);
        helpers.updateMap(clusterUri, draft.kubes, []);
      });

      return;
    }

    this.setState(draft => {
      draft.kubesSyncStatus.set(clusterUri, {
        status: 'processing',
      });
    });

    try {
      const received = await this.client.listKubes(clusterUri);
      this.setState(draft => {
        draft.kubesSyncStatus.set(clusterUri, { status: 'ready' });
        helpers.updateMap(clusterUri, draft.kubes, received);
      });
    } catch (err) {
      this.setState(draft => {
        draft.kubesSyncStatus.set(clusterUri, {
          status: 'failed',
          statusText: err.message,
        });
      });
    }
  }

  async syncApps(clusterUri: string) {
    const cluster = this.state.clusters.get(clusterUri);
    if (!cluster.connected) {
      this.setState(draft => {
        draft.appsSyncStatus.delete(clusterUri);
        helpers.updateMap(clusterUri, draft.apps, []);
      });

      return;
    }

    this.setState(draft => {
      draft.appsSyncStatus.set(clusterUri, {
        status: 'processing',
      });
    });

    try {
      const received = await this.client.listApps(clusterUri);
      this.setState(draft => {
        draft.appsSyncStatus.set(clusterUri, { status: 'ready' });
        helpers.updateMap(clusterUri, draft.apps, received);
      });
    } catch (err) {
      this.setState(draft => {
        draft.appsSyncStatus.set(clusterUri, {
          status: 'failed',
          statusText: err.message,
        });
      });
    }
  }

  async syncDbs(clusterUri: string) {
    const cluster = this.state.clusters.get(clusterUri);
    if (!cluster.connected) {
      this.setState(draft => {
        draft.dbsSyncStatus.delete(clusterUri);
        helpers.updateMap(clusterUri, draft.dbs, []);
      });

      return;
    }

    this.setState(draft => {
      draft.dbsSyncStatus.set(clusterUri, {
        status: 'processing',
      });
    });

    try {
      const received = await this.client.listDatabases(clusterUri);
      this.setState(draft => {
        draft.dbsSyncStatus.set(clusterUri, { status: 'ready' });
        helpers.updateMap(clusterUri, draft.dbs, received);
      });
    } catch (err) {
      this.setState(draft => {
        draft.dbsSyncStatus.set(clusterUri, {
          status: 'failed',
          statusText: err.message,
        });
      });
    }
  }

  async syncLeafClusters(clusterUri: string) {
    const leaves = await this.syncLeafClustersList(clusterUri);

    leaves
      .filter(c => c.connected)
      .forEach(c => this.syncLeafClusterResourcesAndCatchErrors(c.uri));
  }

  private async syncLeafClustersList(clusterUri: string) {
    const leaves = await this.client.listLeafClusters(clusterUri);

    this.setState(draft => {
      for (const leaf of leaves) {
        draft.clusters.set(
          leaf.uri,
          this.removeInternalLoginsFromCluster(leaf)
        );
      }
    });

    return leaves;
  }

  async syncServers(clusterUri: string) {
    const cluster = this.state.clusters.get(clusterUri);
    if (!cluster.connected) {
      this.setState(draft => {
        draft.serversSyncStatus.delete(clusterUri);
        helpers.updateMap(clusterUri, draft.servers, []);
      });

      return;
    }

    this.setState(draft => {
      draft.serversSyncStatus.set(clusterUri, {
        status: 'processing',
      });
    });

    try {
      const received = await this.client.listServers(clusterUri);
      this.setState(draft => {
        draft.serversSyncStatus.set(clusterUri, { status: 'ready' });
        helpers.updateMap(clusterUri, draft.servers, received);
      });
    } catch (err) {
      this.setState(draft => {
        draft.serversSyncStatus.set(clusterUri, {
          status: 'failed',
          statusText: err.message,
        });
      });
    }
  }

  /**
   * Removes cluster and its leaf clusters (if any)
   */
  async removeCluster(clusterUri: string) {
    await this.client.removeCluster(clusterUri);
    const leafClustersUris = this.getClusters()
      .filter(
        item =>
          item.leaf && routing.ensureRootClusterUri(item.uri) === clusterUri
      )
      .map(cluster => cluster.uri);
    this.setState(draft => {
      draft.clusters.delete(clusterUri);
      leafClustersUris.forEach(leafClusterUri => {
        draft.clusters.delete(leafClusterUri);
      });
    });

    this.removeResources(clusterUri);
    leafClustersUris.forEach(leafClusterUri => {
      this.removeResources(leafClusterUri);
    });
  }

  async getAuthSettings(clusterUri: string) {
    return (await this.client.getAuthSettings(clusterUri)) as AuthSettings;
  }

  async createGateway(params: CreateGatewayParams) {
    const gateway = await this.client.createGateway(params);
    this.setState(draft => {
      draft.gateways.set(gateway.uri, gateway);
    });
    return gateway;
  }

  async removeGateway(gatewayUri: string) {
    try {
      await this.client.removeGateway(gatewayUri);
      this.setState(draft => {
        draft.gateways.delete(gatewayUri);
      });
    } catch (error) {
      const gateway = this.findGateway(gatewayUri);
      const gatewayDescription = gateway
        ? `for ${gateway.targetUser}@${gateway.targetName}`
        : gatewayUri;
      const title = `Could not close the database connection ${gatewayDescription}`;

      this.notificationsService.notifyError({
        title,
        description: error.message,
      });
      throw error;
    }
  }

  async restartGateway(gatewayUri: string) {
    await this.client.restartGateway(gatewayUri);
  }

  async setGatewayTargetSubresourceName(
    gatewayUri: string,
    targetSubresourceName: string
  ) {
    if (!this.findGateway(gatewayUri)) {
      throw new Error(`Could not find gateway ${gatewayUri}`);
    }

    const gateway = await this.client.setGatewayTargetSubresourceName(
      gatewayUri,
      targetSubresourceName
    );

    this.setState(draft => {
      draft.gateways.set(gatewayUri, gateway);
    });

    return gateway;
  }

  async setGatewayLocalPort(gatewayUri: string, localPort: string) {
    if (!this.findGateway(gatewayUri)) {
      throw new Error(`Could not find gateway ${gatewayUri}`);
    }

    const gateway = await this.client.setGatewayLocalPort(
      gatewayUri,
      localPort
    );

    this.setState(draft => {
      draft.gateways.set(gatewayUri, gateway);
    });

    return gateway;
  }

  findCluster(clusterUri: string) {
    return this.state.clusters.get(clusterUri);
  }

  findDbs(clusterUri: string) {
    return [...this.state.dbs.values()].filter(db =>
      routing.isClusterDb(clusterUri, db.uri)
    );
  }

  findGateway(gatewayUri: string) {
    return this.state.gateways.get(gatewayUri);
  }

  findDb(dbUri: string) {
    return this.state.dbs.get(dbUri);
  }

  findApps(clusterUri: string) {
    return [...this.state.apps.values()].filter(s =>
      routing.isClusterApp(clusterUri, s.uri)
    );
  }

  findKubes(clusterUri: string) {
    return [...this.state.kubes.values()].filter(s =>
      routing.isClusterKube(clusterUri, s.uri)
    );
  }

  findServers(clusterUri: string) {
    return [...this.state.servers.values()].filter(s =>
      routing.isClusterServer(clusterUri, s.uri)
    );
  }

  findGateways(clusterUri: string) {
    return [...this.state.gateways.values()].filter(s =>
      routing.belongsToProfile(clusterUri, s.targetUri)
    );
  }

  findClusterByResource(uri: string) {
    const parsed = routing.parseClusterUri(uri);
    if (!parsed) {
      return null;
    }

    const clusterUri = routing.getClusterUri(parsed.params);
    return this.findCluster(clusterUri);
  }

  findRootClusterByResource(uri: string) {
    const parsed = routing.parseClusterUri(uri);
    if (!parsed) {
      return null;
    }

    const rootClusterUri = routing.getClusterUri({
      rootClusterId: parsed.params.rootClusterId,
    });
    return this.findCluster(rootClusterUri);
  }

  getServer(serverUri: string) {
    return this.state.servers.get(serverUri);
  }

  getGateways() {
    return [...this.state.gateways.values()];
  }

  getClusters() {
    return [...this.state.clusters.values()];
  }

  getClusterSyncStatus(clusterUri: string) {
    const empty: SyncStatus = { status: '' };
    const dbs = this.state.dbsSyncStatus.get(clusterUri) || empty;
    const servers = this.state.serversSyncStatus.get(clusterUri) || empty;
    const apps = this.state.appsSyncStatus.get(clusterUri) || empty;
    const kubes = this.state.kubesSyncStatus.get(clusterUri) || empty;

    const syncing =
      dbs.status === 'processing' ||
      servers.status === 'processing' ||
      apps.status === 'processing' ||
      kubes.status === 'processing';

    return {
      syncing,
      dbs,
      servers,
      apps,
      kubes,
    };
  }

  getServers() {
    return [...this.state.servers.values()];
  }

  getDbs() {
    return [...this.state.dbs.values()];
  }

  async getDbUsers(dbUri: string): Promise<string[]> {
    return await this.client.listDatabaseUsers(dbUri);
  }

  useState() {
    return useStore(this).state;
  }

  // Note: client.getCluster ultimately reads data from the disk, so syncClusterInfo will not fail
  // with a retryable error in case the certs have expired.
  private async syncClusterInfo(clusterUri: string) {
    const cluster = await this.client.getCluster(clusterUri);
    this.setState(draft => {
      draft.clusters.set(
        clusterUri,
        this.removeInternalLoginsFromCluster(cluster)
      );
    });
  }

  private removeResources(clusterUri: string) {
    this.setState(draft => {
      this.findDbs(clusterUri).forEach(db => {
        draft.dbs.delete(db.uri);
      });

      this.findServers(clusterUri).forEach(server => {
        draft.servers.delete(server.uri);
      });

      this.findApps(clusterUri).forEach(app => {
        draft.apps.delete(app.uri);
      });

      this.findKubes(clusterUri).forEach(kube => {
        draft.kubes.delete(kube.uri);
      });

      draft.serversSyncStatus.delete(clusterUri);
      draft.dbsSyncStatus.delete(clusterUri);
      draft.kubesSyncStatus.delete(clusterUri);
      draft.appsSyncStatus.delete(clusterUri);
    });
  }

  searchDbs(clusterUri: string, query: SearchQuery) {
    const databases = this.findDbs(clusterUri);
    return databases.filter(obj =>
      isMatch(obj, query.search, {
        searchableProps: ['name', 'desc', 'labelsList'],
        cb: (targetValue, searchValue, propName) => {
          if (propName === 'labelsList') {
            return this._isIncludedInTagTargetValue(targetValue, searchValue);
          }
        },
      })
    );
  }

  searchApps(clusterUri: string, query: SearchQuery) {
    const apps = this.findApps(clusterUri);
    return apps.filter(obj =>
      isMatch(obj, query.search, {
        searchableProps: ['name', 'publicAddr', 'description', 'labelsList'],
        cb: (targetValue, searchValue, propName) => {
          if (propName === 'labelsList') {
            return this._isIncludedInTagTargetValue(targetValue, searchValue);
          }
        },
      })
    );
  }

  searchClusters(value: string) {
    const clusters = this.getClusters();
    return clusters.filter(s => {
      return [s.name].join('').toLocaleLowerCase().includes(value);
    });
  }

  searchKubes(clusterUri: string, query: SearchQuery) {
    const kubes = this.findKubes(clusterUri);
    return kubes.filter(obj =>
      isMatch(obj, query.search, {
        searchableProps: ['name', 'labelsList'],
        cb: (targetValue, searchValue, propName) => {
          if (propName === 'labelsList') {
            return this._isIncludedInTagTargetValue(targetValue, searchValue);
          }
        },
      })
    );
  }

  searchServers(clusterUri: string, query: SearchQueryWithProps<tsh.Server>) {
    const servers = this.findServers(clusterUri);
    const searchableProps = query.searchableProps || [
      'hostname',
      'addr',
      'labelsList',
      'tunnel',
    ];
    return servers.filter(obj =>
      isMatch(obj, query.search, {
        searchableProps: searchableProps,
        cb: (targetValue, searchValue, propName) => {
          if (propName === 'tunnel') {
            return 'TUNNEL'.includes(searchValue);
          }

          if (propName === 'labelsList') {
            return this._isIncludedInTagTargetValue(targetValue, searchValue);
          }
        },
      })
    );
  }

  private _isIncludedInTagTargetValue(
    targetValue: Label[],
    searchValue: string
  ) {
    return targetValue.some(item =>
      makeLabelTag(item).toLocaleUpperCase().includes(searchValue)
    );
  }

  // temporary fix for https://github.com/gravitational/webapps.e/issues/294
  // remove when it will get fixed in `tsh`
  // alternatively, show only valid logins basing on RBAC check
  private removeInternalLoginsFromCluster(cluster: Cluster): Cluster {
    return {
      ...cluster,
      loggedInUser: cluster.loggedInUser && {
        ...cluster.loggedInUser,
        sshLoginsList: cluster.loggedInUser.sshLoginsList.filter(
          login => !login.startsWith('-')
        ),
      },
    };
  }
}

type SearchQuery = {
  search: string;
};

type SearchQueryWithProps<T> = SearchQuery & {
  searchableProps?: (keyof T)[];
};

const helpers = {
  updateMap<T extends { uri: string }>(
    parentUri = '',
    map: Map<string, T>,
    received: T[]
  ) {
    // delete all entries under given uri
    for (let k of map.keys()) {
      if (routing.ensureClusterUri(k) === parentUri) {
        map.delete(k);
      }
    }

    received.forEach(s => map.set(s.uri, s));
  },
};
