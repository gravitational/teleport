/*
Copyright 2020 Gravitational, Inc.

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

import { useEffect } from 'react';
import { useAppContext } from 'teleterm/ui/appContextProvider';
import { IAppContext } from 'teleterm/ui/types';
import * as types from 'teleterm/ui/services/workspacesService';
import { DocumentsService } from 'teleterm/ui/services/workspacesService';
import { PtyCommand, PtyProcess } from 'teleterm/services/pty/types';
import useAsync from 'teleterm/ui/useAsync';
import { useWorkspaceDocumentsService } from 'teleterm/ui/Documents';

export default function useDocumentTerminal(doc: Doc) {
  const ctx = useAppContext();
  const workspaceDocumentsService = useWorkspaceDocumentsService();
  const [state, init] = useAsync(async () =>
    initState(ctx, workspaceDocumentsService, doc),
  );

  useEffect(() => {
    init();
    return () => {
      state.data?.ptyProcess.dispose();
    };
  }, []);

  return state;
}

async function initState(
  ctx: IAppContext,
  docsService: DocumentsService,
  doc: Doc,
) {
  const cmd = createCmd(doc);
  let ptyProcess: PtyProcess;
  try {
    ptyProcess = await ctx.terminalsService.createPtyProcess(cmd);
  } catch (e) {
    ctx.notificationsService.notifyError(e.message);
    return;
  }
  const openContextMenu = () => ctx.mainProcessClient.openTerminalContextMenu();

  const refreshTitle = async () => {
    if (cmd.kind !== 'pty.shell') {
      return;
    }

    const cwd = await ptyProcess.getCwd();
    docsService.update(doc.uri, { cwd, title: cwd });
  };

  const removeInitCommand = () => {
    if (doc.kind !== 'doc.terminal_shell') {
      return;
    }
    // The initCommand has to be launched only once, not every time we recreate the document from
    // the state.
    docsService.update(doc.uri, { initCommand: undefined });
  };

  ptyProcess.onOpen(() => {
    docsService.update(doc.uri, { status: 'connected' });
    refreshTitle();
    removeInitCommand();
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
      docsService.close(doc.uri);
    }
  });

  return {
    ptyProcess,
    refreshTitle,
    openContextMenu,
  };
}

function createCmd(doc: Doc): PtyCommand {
  if (doc.kind === 'doc.terminal_tsh_node') {
    return {
      ...doc,
      kind: 'pty.tsh-login',
    };
  }

  if (doc.kind === 'doc.terminal_tsh_kube') {
    return {
      ...doc,
      kind: 'pty.tsh-kube-login',
    };
  }

  return {
    kind: 'pty.shell',
    cwd: doc.cwd,
    initCommand: doc.initCommand,
  };
}

type Doc = types.DocumentTerminal;

export type Props = {
  doc: Doc;
  visible: boolean;
};
