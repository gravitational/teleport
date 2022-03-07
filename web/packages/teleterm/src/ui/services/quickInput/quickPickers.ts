/*
Copyright 2019 Gravitational, Inc.

Licensed under the Apache License, Version 2.0 (the "License");
you may not use this file except in compliance with the License.
You may obtain a copy of the License at

    http://www.apache.org/licenses/LICENSE-2.0

Unless required by applicable law or agreed to in writing, software
distributed under the License is distributed on an "AS IS" BASIS,
WITHOUT WARRANTIES OR CONDITIONS OF ANY KIND, either express or implied.
See the License for the specific language governing permissions and
limitations under the License.
*/

import { ClustersService } from 'teleterm/ui/services/clusters';
import { CommandLauncher } from 'teleterm/ui/commandLauncher';
import {
  QuickInputPicker,
  Item,
  ItemCmd,
  ItemServer,
  ItemDb,
  ItemNewCluster,
  ItemCluster,
} from './types';

export abstract class ClusterPicker implements QuickInputPicker {
  abstract onFilter(value: string): Item[];
  abstract onPick(result: Item): void;

  launcher: CommandLauncher;
  serviceCluster: ClustersService;

  constructor(launcher: CommandLauncher, service: ClustersService) {
    this.serviceCluster = service;
    this.launcher = launcher;
  }

  protected searchClusters(value: string): Item[] {
    const clusters = this.serviceCluster.searchClusters(value);
    const items: ItemCluster[] = clusters
      .filter(s => !s.leaf)
      .map(cluster => {
        return {
          kind: 'item.cluster',
          data: cluster,
        };
      });

    return ensureEmptyPlaceholder(items);
  }

  protected searchServers(value: string): Item[] {
    const clusters = this.serviceCluster.getClusters();
    const items: Item[] = [];
    for (const { uri } of clusters) {
      const servers = this.serviceCluster.searchServers(uri, { search: value });
      for (const server of servers) {
        items.push({
          kind: 'item.server',
          data: server,
        });
      }
    }
    return ensureEmptyPlaceholder(items);
  }

  protected searchDbs(value: string): Item[] {
    const clusters = this.serviceCluster.getClusters();
    const items: Item[] = [];
    for (const { uri } of clusters) {
      const dbs = this.serviceCluster.searchDbs(uri, { search: value });
      for (const db of dbs) {
        items.push({
          kind: 'item.db',
          data: db,
        });
      }
    }
    return ensureEmptyPlaceholder(items);
  }
}

export class QuickLoginPicker extends ClusterPicker {
  onFilter(value = '') {
    const items = this.searchClusters(value);
    if (value === '') {
      const addNew: ItemNewCluster = {
        kind: 'item.cluster-new',
        data: {
          displayName: 'new cluster...',
          description: 'Enter a new cluster name to login',
        },
      };

      items.unshift(addNew);
    }

    return items;
  }

  onPick(item: ItemCluster | ItemNewCluster) {
    this.launcher.executeCommand('cluster-connect', {
      clusterUri: item.data.uri,
    });
  }
}

export class QuickDbPicker extends ClusterPicker {
  onFilter(value = '') {
    return this.searchDbs(value);
  }

  onPick(item: ItemDb) {
    this.launcher.executeCommand('proxy-db', {
      dbUri: item.data.uri,
    });
  }
}

export class QuickServerPicker extends ClusterPicker {
  onFilter(value = '') {
    return this.searchServers(value);
  }

  onPick(item: ItemServer) {
    this.launcher.executeCommand('ssh', {
      serverUri: item.data.uri,
    });
  }
}

export class QuickCommandPicker implements QuickInputPicker {
  onFilter() {
    return [];
  }

  onPick() {}
}

function ensureEmptyPlaceholder(items: Item[]): Item[] {
  if (items.length === 0) {
    items.push({ kind: 'item.empty', data: { message: 'not found' } });
  }

  return items;
}
