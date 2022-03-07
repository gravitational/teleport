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
import { WorkspacesService } from 'teleterm/ui/services/workspacesService';
import { CommandLauncher } from 'teleterm/ui/commandLauncher';
import {
  QuickInputPicker,
  Item,
  ItemCmd,
  ItemServer,
  ItemDb,
  ItemNewCluster,
  ItemCluster,
  ItemSshLogin,
  AutocompleteResult,
} from './types';

export abstract class ClusterPicker implements QuickInputPicker {
  abstract onPick(result: Item): void;
  abstract getAutocompleteResult(value: string): AutocompleteResult;

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

  getAutocompleteResult() {
    return null;
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

  getAutocompleteResult() {
    return null;
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

  getAutocompleteResult() {
    return null;
  }
}

export class QuickCommandPicker implements QuickInputPicker {
  private pickerRegistry: Map<string, QuickInputPicker>;

  constructor(private launcher: CommandLauncher) {
    this.pickerRegistry = new Map();
  }

  registerPickerForCommand(command: string, picker: QuickInputPicker) {
    this.pickerRegistry.set(command, picker);
  }

  onPick(item: ItemCmd) {
    this.launcher.executeCommand(item.data.name as any, null);
  }

  // TODO: Add tests.
  // `tsh ssh foo` as input should return recognized-token.
  // TODO(ravicious): Handle env vars.
  getAutocompleteResult(input: string): AutocompleteResult {
    const autocompleteCommands = this.launcher.getAutocompleteCommands();

    // We can safely ignore any whitespace at the start.
    input = input.trimStart();

    // Return all commands if there's no input.
    if (input === '') {
      return {
        kind: 'autocomplete.partial-match',
        picker: this,
        listItems: this.mapAutocompleteCommandsToItems(autocompleteCommands),
      };
    }

    const matchingAutocompleteCommands = this.launcher
      .getAutocompleteCommands()
      .filter(cmd => {
        const completeMatchRegex = new RegExp(`^${cmd.displayName}\\b`);
        return (
          cmd.displayName.startsWith(input) ||
          // `completeMatchRegex` handles situations where the `input` akin to "tsh ssh foo".
          // In that case, "tsh ssh" is the matching command, even though
          // `cmd.displayName` ("tsh ssh") doesn't start with `input` ("tsh ssh foo").
          completeMatchRegex.test(input)
        );
      });

    if (matchingAutocompleteCommands.length === 0) {
      return { kind: 'autocomplete.no-match', picker: this };
    }

    if (matchingAutocompleteCommands.length > 1) {
      return {
        kind: 'autocomplete.partial-match',
        picker: this,
        listItems: this.mapAutocompleteCommandsToItems(
          matchingAutocompleteCommands
        ),
      };
    }

    // The rest of the function body handles situation in which there's only one matching
    // autocomplete command.

    // Handles a complete match, for example the input is `tsh ssh`.
    const soleMatch = matchingAutocompleteCommands[0];
    const completeMatchRegex = new RegExp(`^${soleMatch.displayName}\\b`);
    const isCompleteMatch = completeMatchRegex.test(input);

    if (isCompleteMatch) {
      const valueWithoutCommandPrefix = input.replace(completeMatchRegex, '');
      const nextQuickInputPicker = this.pickerRegistry.get(
        soleMatch.displayName
      );
      return nextQuickInputPicker.getAutocompleteResult(
        valueWithoutCommandPrefix
      );
    }

    // Handles a non-complete match with only a single matching command, for example if the input is
    // `tsh ss` (`tsh ssh` should be the only matching command then).
    return {
      kind: 'autocomplete.partial-match',
      picker: this,
      listItems: this.mapAutocompleteCommandsToItems(
        matchingAutocompleteCommands
      ),
    };
  }

  private mapAutocompleteCommandsToItems(
    commands: { name: string; displayName: string; description: string }[]
  ): ItemCmd[] {
    return commands.map(cmd => ({
      kind: 'item.cmd' as const,
      data: cmd,
    }));
  }
}

export class QuickTshSshPicker implements QuickInputPicker {
  // TODO: Make sure this regex is okay and covers valid usernames.
  loginRegex = /^\s+([a-zA-Z0-9_-]*\b)?$/;

  constructor(
    private launcher: CommandLauncher,
    private sshLoginPicker: QuickSshLoginPicker
  ) {}

  onFilter() {
    return [];
  }

  onPick(item: ItemCmd) {
    // TODO: Execute SSH.
    // this.launcher.executeCommand(item.data.name as any, null);
  }

  // TODO: Support cluster arg.
  getAutocompleteResult(input: string): AutocompleteResult {
    const loginMatch = input.match(this.loginRegex);

    if (loginMatch) {
      return this.sshLoginPicker.getAutocompleteResult(loginMatch[1]);
    }

    return {
      kind: 'autocomplete.no-match',
      picker: this,
    };
  }
}

// TODO: Implement the rest of this class.
export class QuickTshProxyDbPicker implements QuickInputPicker {
  constructor(private launcher: CommandLauncher) {}

  onFilter() {
    return [];
  }

  onPick(item: ItemCmd) {
    // TODO: Execute Proxy db.
    // this.launcher.executeCommand(item.data.name as any, null);
  }

  getAutocompleteResult(input: string): AutocompleteResult {
    return {
      kind: 'autocomplete.no-match',
      picker: this,
    };
  }
}

export class QuickSshLoginPicker implements QuickInputPicker {
  constructor(
    private workspacesService: WorkspacesService,
    private clustersService: ClustersService
  ) {}

  filterSshLogins(input: string): ItemSshLogin[] {
    // TODO(ravicious): Use local cluster URI.
    // TODO(ravicious): Handle the `--cluster` tsh ssh flag.
    const rootClusterUri = this.workspacesService.getRootClusterUri();
    const cluster = this.clustersService.findCluster(rootClusterUri);
    const allLogins = cluster?.loggedInUser?.sshLoginsList || [];
    let matchingLogins: typeof allLogins;

    if (!input) {
      matchingLogins = allLogins;
    } else {
      matchingLogins = allLogins.filter(login => login.startsWith(input));
    }

    return matchingLogins.map(login => ({
      kind: 'item.ssh-login' as const,
      data: login,
    }));
  }

  // TODO: Append the rest of the login to quickInputService's inputValue.
  onPick(item: ItemCmd) {}

  getAutocompleteResult(input: string): AutocompleteResult {
    const listItems = this.filterSshLogins(input);
    return {
      kind: 'autocomplete.partial-match',
      picker: this,
      listItems,
    };
  }
}

function ensureEmptyPlaceholder(items: Item[]): Item[] {
  if (items.length === 0) {
    items.push({ kind: 'item.empty', data: { message: 'not found' } });
  }

  return items;
}
