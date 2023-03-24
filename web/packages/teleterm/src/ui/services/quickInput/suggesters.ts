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

import { WorkspacesService } from 'teleterm/ui/services/workspacesService';
import { ResourcesService } from 'teleterm/ui/services/resources';
import { ClustersService } from 'teleterm/ui/services/clusters';

import {
  SuggestionServer,
  SuggestionSshLogin,
  SuggestionDatabase,
  QuickInputSuggester,
} from './types';

const limit = 10;

export class QuickSshLoginSuggester
  implements QuickInputSuggester<SuggestionSshLogin>
{
  constructor(
    private workspacesService: WorkspacesService,
    private clustersService: ClustersService
  ) {}

  async getSuggestions(input: string): Promise<SuggestionSshLogin[]> {
    // TODO(ravicious): Handle the `--cluster` tsh ssh flag.
    const localClusterUri =
      this.workspacesService.getActiveWorkspace()?.localClusterUri;
    if (!localClusterUri) {
      return [];
    }
    const cluster = this.clustersService.findCluster(localClusterUri);
    const allLogins = cluster?.loggedInUser?.sshLoginsList || [];
    let matchingLogins: typeof allLogins;

    if (!input) {
      matchingLogins = allLogins;
    } else {
      matchingLogins = allLogins.filter(login =>
        login.startsWith(input.toLowerCase())
      );
    }

    return matchingLogins.map(login => ({
      kind: 'suggestion.ssh-login' as const,
      token: login,
      // This allows the user to immediately begin typing the hostname.
      appendToToken: '@',
      data: login,
    }));
  }
}

export class QuickServerSuggester
  implements QuickInputSuggester<SuggestionServer>
{
  constructor(
    private workspacesService: WorkspacesService,
    private resourcesService: ResourcesService
  ) {}

  async getSuggestions(input: string): Promise<SuggestionServer[]> {
    // TODO(ravicious): Handle the `--cluster` tsh ssh flag.
    const localClusterUri =
      this.workspacesService.getActiveWorkspace()?.localClusterUri;
    if (!localClusterUri) {
      return [];
    }
    const servers = await this.resourcesService.fetchServers({
      clusterUri: localClusterUri,
      search: input,
      limit,
    });

    return servers.agentsList.map(server => ({
      kind: 'suggestion.server' as const,
      token: server.hostname,
      data: server,
    }));
  }
}

export class QuickDatabaseSuggester
  implements QuickInputSuggester<SuggestionDatabase>
{
  constructor(
    private workspacesService: WorkspacesService,
    private resourcesService: ResourcesService
  ) {}

  async getSuggestions(input: string): Promise<SuggestionDatabase[]> {
    const localClusterUri =
      this.workspacesService.getActiveWorkspace()?.localClusterUri;
    if (!localClusterUri) {
      return [];
    }
    const databases = await this.resourcesService.fetchDatabases({
      clusterUri: localClusterUri,
      search: input,
      limit,
    });

    return databases.agentsList.map(database => ({
      kind: 'suggestion.database' as const,
      token: database.name,
      data: database,
    }));
  }
}
