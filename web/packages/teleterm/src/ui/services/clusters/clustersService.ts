import { useStore } from 'shared/libs/stores';
import {
  AuthSettings,
  ClustersServiceState,
  CreateGatewayParams,
  LoginParams,
  SyncStatus,
  tsh,
} from './types';
import { ImmutableStore } from '../immutableStore';
import { routing } from 'teleterm/ui/uri';
import isMatch from 'design/utils/match';
import { makeLabelTag } from 'teleport/components/formatters';
import { Label } from 'teleport/types';
import { NotificationsService } from 'teleterm/ui/services/notifications';

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
      draft.clusters.set(cluster.uri, cluster);
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
    await this.syncRootCluster(params.clusterUri);
  }

  async syncRootCluster(clusterUri: string) {
    try {
      await Promise.all([
        this.syncClusterInfo(clusterUri),
        this.syncLeafClusters(clusterUri),
      ]);
    } catch (e) {
      this.notificationsService.notifyError({
        title: `Could not synchronize cluster ${
          routing.parseClusterUri(clusterUri).params.rootClusterId
        }`,
        description: e.message,
      });
    }
    this.syncKubes(clusterUri);
    this.syncApps(clusterUri);
    this.syncDbs(clusterUri);
    this.syncServers(clusterUri);
    this.syncKubes(clusterUri);
    this.syncGateways();
  }

  async syncLeafCluster(clusterUri: string) {
    // do not await these
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
        .forEach(c => this.syncRootCluster(c.uri));
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
      return this.syncLeafCluster(clusterUri);
    } else {
      return this.syncRootCluster(clusterUri);
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
        title: 'Could not fetch databases',
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
    const leaves = await this.client.listLeafClusters(clusterUri);
    this.setState(draft => {
      for (const leaf of leaves) {
        draft.clusters.set(leaf.uri, leaf);
      }
    });

    leaves.filter(c => c.connected).forEach(c => this.syncLeafCluster(c.uri));
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

  async removeCluster(clusterUri: string) {
    await this.client.removeCluster(clusterUri);
    this.setState(draft => {
      draft.clusters.delete(clusterUri);
    });
    this.removeResources(clusterUri);
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
      this.notificationsService.notifyError({
        title: 'Could not close the database connection',
        description: error.message,
      });
      throw error;
    }
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

  useState() {
    return useStore(this).state;
  }

  private async syncClusterInfo(clusterUri: string) {
    const cluster = await this.client.getCluster(clusterUri);
    this.setState(draft => {
      draft.clusters.set(clusterUri, cluster);
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
      if (k.startsWith(parentUri)) {
        map.delete(k);
      }
    }

    received.forEach(s => map.set(s.uri, s));
  },
};
