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

import { IAppContext } from 'teleterm/ui/types';
import { routing } from 'teleterm/ui/uri';
import { tsh } from 'teleterm/ui/services/clusters/types';

const commands = {
  // For handling "tsh ssh" executed from the command bar.
  'tsh-ssh': {
    displayName: '',
    description: '',
    run(
      ctx: IAppContext,
      args: { loginHost: string; localClusterUri: string }
    ) {
      const { loginHost, localClusterUri } = args;
      const rootClusterUri = routing.ensureRootClusterUri(localClusterUri);
      const documentsService =
        ctx.workspacesService.getWorkspaceDocumentService(rootClusterUri);
      let login: string | undefined, host: string;
      const parts = loginHost.split('@');

      if (parts.length > 1) {
        host = parts.pop();
        // If someone types in `foo@bar@baz` as input here, `parts` will have more than two
        // elements. `foo@bar` is probably not a valid login, but we don't want to lose that
        // input here.
        //
        // In any case, we're just repeating what `tsh ssh` is doing with inputs like these.
        login = parts.join('@');
      } else {
        host = parts[0];
      }

      // TODO(ravicious): Handle finding host by more than just a name.
      // Basically we have to replicate tsh ssh behavior here.
      const servers = ctx.clustersService.searchServers(localClusterUri, {
        search: host,
        searchableProps: ['hostname'],
      });
      let server: tsh.Server | undefined;

      if (servers.length === 1) {
        server = servers[0];
      } else if (servers.length > 1) {
        // TODO(ravicious): Handle ambiguous host name. See `onSSH` in `tool/tsh/tsh.go`.
        console.error('Ambiguous host');
      }

      let serverUri: string, serverHostname: string;

      if (server) {
        serverUri = server.uri;
        serverHostname = server.hostname;
      } else {
        // If we can't find a server by the given hostname, we still want to create a document to
        // handle the error further down the line.
        const clusterParams = routing.parseClusterUri(localClusterUri).params;
        serverUri = routing.getServerUri({
          ...clusterParams,
          serverId: host,
        });
        serverHostname = host;
      }
      // TODO(ravicious): Handle failure due to incorrect host name.
      const doc = documentsService.createTshNodeDocument(serverUri);
      doc.title = login ? `${login}@${serverHostname}` : serverHostname;
      doc.login = login;
      documentsService.add(doc);
      documentsService.setLocation(doc.uri);
    },
  },

  'kube-connect': {
    displayName: '',
    description: '',
    run(ctx: IAppContext, args: { kubeUri: string }) {
      const documentsService =
        ctx.workspacesService.getActiveWorkspaceDocumentService();
      const kubeDoc = documentsService.createTshKubeDocument(args.kubeUri);
      documentsService.add(kubeDoc);
      documentsService.open(kubeDoc.uri);
    },
  },

  'cluster-connect': {
    displayName: '',
    description: '',
    run(ctx: IAppContext, args: { clusterUri?: string; onSuccess?(): void }) {
      const defaultHandler = (clusterUri: string) => {
        ctx.commandLauncher.executeCommand('cluster-open', { clusterUri });
      };

      ctx.modalsService.openClusterConnectDialog({
        clusterUri: args.clusterUri,
        onSuccess: args.onSuccess || defaultHandler,
      });
    },
  },

  'cluster-logout': {
    displayName: '',
    description: '',
    run(ctx: IAppContext, args: { clusterUri: string }) {
      const cluster = ctx.clustersService.findCluster(args.clusterUri);
      ctx.modalsService.openDialog({
        kind: 'cluster-logout',
        clusterUri: cluster.uri,
        clusterTitle: cluster.name,
      });
    },
  },

  'cluster-open': {
    displayName: '',
    description: '',
    async run(ctx: IAppContext, args: { clusterUri: string }) {
      const { clusterUri } = args;
      const rootCluster =
        ctx.clustersService.findRootClusterByResource(clusterUri);
      await ctx.workspacesService.setActiveWorkspace(rootCluster.uri);
      const documentsService =
        ctx.workspacesService.getWorkspaceDocumentService(rootCluster.uri);
      const doc = documentsService.findClusterDocument(clusterUri);
      if (doc) {
        documentsService.open(doc.uri);
      } else {
        const newDoc = documentsService.createClusterDocument({ clusterUri });
        documentsService.add(newDoc);
        documentsService.open(newDoc.uri);
      }
    },
  },

  'autocomplete.tsh-ssh': {
    displayName: 'tsh ssh',
    description: 'Run shell or execute a command on a remote SSH node',
    run() {},
  },
  'autocomplete.tsh-proxy-db': {
    displayName: 'tsh proxy db',
    description:
      'Start local TLS proxy for database connections when using Teleport',
    run() {},
  },
};

export class CommandLauncher {
  appContext: IAppContext;

  constructor(appContext: IAppContext) {
    this.appContext = appContext;
  }

  executeCommand<T extends CommandName>(name: T, args: CommandArgs<T>) {
    commands[name].run(this.appContext, args as any);
  }

  getAutocompleteCommands() {
    return Object.entries(commands)
      .filter(([key]) => key.startsWith('autocomplete.'))
      .map(([key, value]) => ({ name: key, ...value }));
  }
}

type CommandName = keyof typeof commands;
type CommandRegistry = typeof commands;
type CommandArgs<T extends CommandName> = Parameters<
  CommandRegistry[T]['run']
>[1];
