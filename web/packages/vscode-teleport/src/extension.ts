import { ExtensionContext, window, workspace } from 'vscode';

import { scheme, TeleportFileSystem } from './TeleportFileSystem';
import { TeleportResourcesTreeProvider } from './TeleportResourcesTreeProvider';

// This method is called when your extension is activated
// Your extension is activated the very first time the command is executed
export function activate(context: ExtensionContext) {
  // Use the console to output diagnostic information (console.log) and errors (console.error)
  // This line of code will only be executed once when your extension is activated
  console.log(
    'Congratulations, your extension "vscode-teleport" is now active!'
  );

  context.subscriptions.push(
    workspace.registerFileSystemProvider(scheme, new TeleportFileSystem())
  );

  context.subscriptions.push(
    window.registerTreeDataProvider(
      'teleportResources',
      new TeleportResourcesTreeProvider()
    )
  );
}

// This method is called when your extension is deactivated
export function deactivate() {}
