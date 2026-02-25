import { exec, execFile } from 'child_process';
import { promisify } from 'util';

import {
  ProviderResult,
  TreeDataProvider,
  TreeItem,
  TreeItemCollapsibleState,
  Uri,
  workspace,
} from 'vscode';

import { scheme } from './TeleportFileSystem';

type TeleportTreeItem = {
  uri: Uri;
};

export class TeleportResourcesTreeProvider implements TreeDataProvider<TeleportTreeItem> {
  getTreeItem(element: TeleportTreeItem): TreeItem | Thenable<TreeItem> {
    return new TreeItem(element.uri, TreeItemCollapsibleState.Collapsed);
  }
  async getChildren(
    element?: TeleportTreeItem | undefined
  ): Promise<TeleportTreeItem[]> {
    const uri = element?.uri ?? Uri.from({ scheme, path: '/' });
    const dir = await workspace.fs.readDirectory(uri);

    return dir.map(([name]) => ({
      uri: Uri.joinPath(uri, name),
    }));
  }
}
