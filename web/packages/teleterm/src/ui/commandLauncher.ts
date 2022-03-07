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

const commands = {
  ssh: {
    displayName: '',
    description: '',
    run(ctx: IAppContext, args: { serverUri: string }) {
      ctx.modalsService.openProxySshDialog(args.serverUri);
    },
  },
  'proxy-db': {
    displayName: '',
    description: '',
    run(
      ctx: IAppContext,
      args: {
        dbUri: string;
        port?: string;
        onSuccess?(gatewayUri: string): void;
      }
    ) {
      const onSuccess = (gatewayUri: string) => {
        const documentsService =
          ctx.workspacesService.getActiveWorkspaceDocumentService();
        const db = ctx.clustersService.findDb(args.dbUri);
        const gateway = ctx.clustersService.findGateway(gatewayUri);
        const doc = documentsService.createGatewayDocument({
          title: db.name,
          gatewayUri: gatewayUri,
          targetUri: gateway.targetUri,
          targetUser: gateway.targetUser,
          port: gateway.localPort,
        });

        documentsService.add(doc);
        documentsService.open(doc.uri);
      };

      ctx.modalsService.openProxyDbDialog({
        dbUri: args.dbUri,
        port: args.port,
        onSuccess: args.onSuccess || onSuccess,
      });
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

      ctx.modalsService.openClusterConnectDialog(
        args.clusterUri,
        args.onSuccess || defaultHandler
      );
    },
  },

  'cluster-remove': {
    displayName: '',
    description: '',
    run(ctx: IAppContext, args: { clusterUri: string }) {
      const cluster = ctx.clustersService.findCluster(args.clusterUri);
      ctx.modalsService.openDialog({
        kind: 'cluster-remove',
        clusterUri: cluster.uri,
        clusterTitle: cluster.name,
      });
    },
  },

  'cluster-open': {
    displayName: '',
    description: '',
    run(ctx: IAppContext, args: { clusterUri: string }) {
      const { clusterUri } = args;
      const documentsService =
        ctx.workspacesService.getActiveWorkspaceDocumentService();
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
