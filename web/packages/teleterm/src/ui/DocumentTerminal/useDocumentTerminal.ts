/**
 * Teleport
 * Copyright (C) 2023  Gravitational, Inc.
 *
 * This program is free software: you can redistribute it and/or modify
 * it under the terms of the GNU Affero General Public License as published by
 * the Free Software Foundation, either version 3 of the License, or
 * (at your option) any later version.
 *
 * This program is distributed in the hope that it will be useful,
 * but WITHOUT ANY WARRANTY; without even the implied warranty of
 * MERCHANTABILITY or FITNESS FOR A PARTICULAR PURPOSE.  See the
 * GNU Affero General Public License for more details.
 *
 * You should have received a copy of the GNU Affero General Public License
 * along with this program.  If not, see <http://www.gnu.org/licenses/>.
 */

import { useEffect } from 'react';

import { useAsync } from 'shared/hooks/useAsync';
import { runOnce } from 'shared/utils/highbar';

import type { Shell } from 'teleterm/mainProcess/shell';
import {
  PtyCommand,
  PtyProcessCreationStatus,
  WindowsPty,
} from 'teleterm/services/pty';
import * as tshdGateway from 'teleterm/services/tshd/gateway';
import { IPtyProcess } from 'teleterm/sharedProcess/ptyHost';
import { useAppContext } from 'teleterm/ui/appContextProvider';
import { useWorkspaceContext } from 'teleterm/ui/Documents';
import { ClustersService } from 'teleterm/ui/services/clusters';
import {
  canDocChangeShell,
  DocumentsService,
} from 'teleterm/ui/services/workspacesService';
import type * as types from 'teleterm/ui/services/workspacesService';
import { IAppContext } from 'teleterm/ui/types';
import { routing } from 'teleterm/ui/uri';

export function useDocumentTerminal(doc: types.DocumentTerminal) {
  const ctx = useAppContext();
  const { documentsService } = useWorkspaceContext();
  const [attempt, runAttempt] = useAsync(async () => {
    if ('status' in doc) {
      documentsService.update(doc.uri, { status: 'connecting' });
    }

    // Add `shellId` before going further.
    // When a new document is crated, its `shellId` is empty
    // (setting the default shell would require reading it from ConfigService
    // in DocumentsService and I wasn't sure about adding more dependencies there).
    // Because of that, I decided to initialize this property later.
    // `doc.shellId` is used in here, in `useDocumentTerminal` and in `tabContextMenu`.
    let docWithDefaultShell: types.DocumentTerminal;
    if (canDocChangeShell(doc) && !doc.shellId) {
      docWithDefaultShell = {
        ...doc,
        shellId: ctx.configService.get('terminal.shell').value,
      };
      documentsService.update(doc.uri, docWithDefaultShell);
    }

    try {
      return await setUpPtyProcess(
        ctx,
        documentsService,
        docWithDefaultShell || doc
      );
    } catch (err) {
      if ('status' in doc) {
        documentsService.update(doc.uri, { status: 'error' });
      }

      throw err;
    }
  });

  useEffect(() => {
    if (attempt.status === '') {
      runAttempt();
    }

    return () => {
      if (attempt.status === 'success') {
        void attempt.data.ptyProcess.dispose();
      }
    };
    // This cannot be run only mount. If the user has initialized a new PTY process by clicking the
    // Reconnect button (which happens post mount), we want to dispose this process when
    // DocumentTerminal gets unmounted. To do this, we need to have a fresh reference to ptyProcess.
  }, [attempt]);

  return { attempt, initializePtyProcess: runAttempt };
}

async function setUpPtyProcess(
  ctx: IAppContext,
  documentsService: DocumentsService,
  doc: types.DocumentTerminal
) {
  const getClusterName = () => {
    const cluster = ctx.clustersService.findCluster(clusterUri);
    if (cluster) {
      return cluster.name;
    }

    /*
     When restoring the documents, we do not always have the leaf clusters already fetched.
     In that case we can fall back to `clusterId` from a leaf cluster URI
     (for a leaf cluster `clusterId` === `name`)
    */
    const parsed = routing.parseClusterUri(clusterUri);

    if (!parsed?.params?.leafClusterId) {
      throw new Error(
        'The leaf cluster URI was expected, but the URI does not contain the leaf cluster ID'
      );
    }
    return parsed.params.leafClusterId;
  };

  const clusterUri = routing.getClusterUri({
    rootClusterId: doc.rootClusterId,
    leafClusterId: doc.leafClusterId,
  });
  const rootCluster = ctx.clustersService.findRootClusterByResource(clusterUri);
  const cmd = createCmd(
    ctx.clustersService,
    doc,
    rootCluster.proxyHost,
    getClusterName()
  );

  const {
    process: ptyProcess,
    windowsPty,
    shell,
  } = await createPtyProcess(ctx, cmd);
  // Update the document with the shell that was resolved.
  // This may be a different shell than the one passed as `shellId`
  // (for example, if it is no longer available, the default one will be opened).
  documentsService.update(doc.uri, { shellId: shell.id });

  if (doc.kind === 'doc.terminal_tsh_node') {
    ctx.usageService.captureProtocolUse({
      uri: clusterUri,
      protocol: 'ssh',
      origin: doc.origin,
      accessThrough: 'proxy_service',
    });
  }
  if (doc.kind === 'doc.gateway_kube') {
    ctx.usageService.captureProtocolUse({
      uri: clusterUri,
      // Other gateways send one protocol use event per gateway being created, but here we're
      // sending one event per kube tab being opened. In the context of protocol usage, that is fine
      // since we now count not protocol _uses_ but protocol _users_.
      protocol: 'kube',
      origin: doc.origin,
      // This will likely need to be adjusted after adding kube support to VNet. VNet is probably
      // going to send one protocol use event per kube cluster seen, but Connect sends one event per
      // tab opened.
      accessThrough: 'local_proxy',
    });
  }

  const openContextMenu = () => ctx.mainProcessClient.openTerminalContextMenu();

  const refreshTitle = async () => {
    documentsService.refreshPtyTitle(doc.uri, {
      shell: shell,
      cwd: await ptyProcess.getCwd(),
      clusterName: getClusterName(),
      runtimeSettings: ctx.mainProcessClient.getRuntimeSettings(),
    });
  };

  // We don't need to clean up the listeners added on ptyProcess in this function. The effect which
  // calls setUpPtyProcess automatically disposes of the process on cleanup, removing all listeners.
  ptyProcess.onOpen(() => {
    refreshTitle();
  });

  // TODO(ravicious): Refactor runOnce to not use the `n` variable. Otherwise runOnce subtracts 1
  // from n each time the resulting function is executed, which in this context means each time data
  // is transferred from PTY.
  const markDocumentAsConnectedOnce = runOnce(() => {
    if ('status' in doc) {
      documentsService.update(doc.uri, { status: 'connected' });
    }
  });

  // mark document as connected when first data arrives
  ptyProcess.onData(() => markDocumentAsConnectedOnce());

  ptyProcess.onStartError(() => {
    if ('status' in doc) {
      documentsService.update(doc.uri, { status: 'error' });
    }
  });

  ptyProcess.onExit(event => {
    // Not closing the tab on non-zero exit code lets us show the error to the user if, for example,
    // tsh ssh cannot connect to the given node.
    //
    // The downside of this is that if you open a local shell, then execute a command that fails
    // (for example, `cd` to a nonexistent directory), and then try to execute `exit` or press
    // Ctrl + D, the tab won't automatically close, because the last exit code is not zero.
    //
    // We can look up how the terminal in vscode handles this problem, since in the scenario
    // described above they do close the tab correctly.
    if (event.exitCode === 0) {
      documentsService.close(doc.uri);
    }
  });

  return {
    ptyProcess,
    refreshTitle,
    openContextMenu,
    windowsPty,
  };
}

async function createPtyProcess(
  ctx: IAppContext,
  cmd: PtyCommand
): Promise<{
  process: IPtyProcess;
  windowsPty: WindowsPty;
  shell: Shell;
}> {
  const { process, creationStatus, windowsPty, shell } =
    await ctx.terminalsService.createPtyProcess(cmd);

  if (creationStatus === PtyProcessCreationStatus.ResolveShellEnvTimeout) {
    ctx.notificationsService.notifyWarning({
      title: 'Could not source environment variables for shell session',
      description:
        "In order to source the environment variables, a new temporary shell session is opened and then immediately closed, but it didn't close within 10 seconds. " +
        'This most likely means that your shell startup took longer to execute or that your shell waits for an input during startup. \nPlease check your startup files.',
    });
  }

  if (
    cmd.kind === 'pty.shell' &&
    creationStatus === PtyProcessCreationStatus.ShellNotResolved
  ) {
    ctx.notificationsService.notifyWarning({
      title: `Requested shell "${cmd.shellId}" is not available`,
    });
  }

  return { process, windowsPty, shell };
}

// TODO(ravicious): Instead of creating cmd within useDocumentTerminal, make useDocumentTerminal
// accept it as an argument. This will allow components such as DocumentGatewayCliClient contain
// the logic related to their specific use case.
//
// useDocumentTerminal used to assume that the doc contains everything that's needed to create the
// cmd. In case of the gateway CLI client that's not true – the state of ClustersService needs to be
// inspected to get the correct command for the gateway CLI client.
function createCmd(
  clustersService: ClustersService,
  doc: types.DocumentTerminal,
  proxyHost: string,
  clusterName: string
): PtyCommand {
  if (doc.kind === 'doc.terminal_tsh_node') {
    return {
      kind: 'pty.tsh-login',
      proxyHost,
      clusterName,
      login: doc.login,
      serverId: doc.serverId,
      rootClusterId: doc.rootClusterId,
      leafClusterId: doc.leafClusterId,
    };
  }

  if (doc.kind === 'doc.gateway_cli_client') {
    const gateway = clustersService.findGatewayByConnectionParams({
      targetUri: doc.targetUri,
      targetUser: doc.targetUser,
    });
    if (!gateway) {
      // This shouldn't happen as DocumentGatewayCliClient doesn't render DocumentTerminal before
      // the gateway is found. In any case, if it does happen for some reason, the user will see
      // this message and will be able to retry starting the terminal.
      throw new Error(
        `No gateway found for ${doc.targetUser} on ${doc.targetUri}`
      );
    }

    // Below we convert cliCommand fields from Go conventions to Node.js conventions.
    const args = tshdGateway.getCliCommandArgs(gateway.gatewayCliCommand);
    const env = tshdGateway.getCliCommandEnv(gateway.gatewayCliCommand);
    // We must not use argsList[0] as the path. Windows expects the executable to end with `.exe`,
    // so if we passed just `psql` here, we wouldn't be able to start the process.
    //
    // Instead, let's use the absolute path resolved by Go.
    const path = gateway.gatewayCliCommand.path;

    return {
      kind: 'pty.gateway-cli-client',
      path,
      args,
      env,
      proxyHost,
      clusterName,
    };
  }

  if (doc.kind === 'doc.gateway_kube') {
    const gateway = clustersService.findGatewayByConnectionParams({
      targetUri: doc.targetUri,
    });
    if (!gateway) {
      throw new Error(`No gateway found for ${doc.targetUri}`);
    }

    const env = tshdGateway.getCliCommandEnv(gateway.gatewayCliCommand);

    if ('KUBECONFIG' in env === false) {
      // This shouldn't happen as 'KUBECONFIG' is the sole purpose of the CLI
      // command for a kube gateway.
      throw new Error(
        `No KUBECONFIG provided for gateway ${gateway.targetUri}`
      );
    }
    const initMessage =
      `Started a local proxy for Kubernetes cluster "${gateway.targetName}".\r\n\r\n` +
      'The KUBECONFIG env var can be used with third-party tools as long as the proxy is running.\r\n' +
      'Close the proxy from Connections in the top left corner or by closing Teleport Connect.\r\n\r\n' +
      'Try "kubectl version" to test the connection.\r\n\r\n';

    return {
      kind: 'pty.shell',
      proxyHost,
      clusterName,
      env,
      initMessage,
      shellId: doc.shellId,
    };
  }

  return {
    ...doc,
    kind: 'pty.shell',
    proxyHost,
    clusterName,
    cwd: doc.cwd,
    shellId: doc.shellId,
  };
}
