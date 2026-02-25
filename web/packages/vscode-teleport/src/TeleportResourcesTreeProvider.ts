import { exec, execFile } from 'child_process';
import { promisify } from 'util';

import {
  FileType,
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
  fileType: FileType;
};

export class TeleportResourcesTreeProvider implements TreeDataProvider<TeleportTreeItem> {
  getTreeItem(element: TeleportTreeItem): TreeItem | Thenable<TreeItem> {
    const ti = new TreeItem(
      element.uri,
      element.fileType === FileType.Directory
        ? TreeItemCollapsibleState.Collapsed
        : TreeItemCollapsibleState.None
    );
    ti.command = {
      command: 'teleport.openResourceAsYaml',
      title: 'Open',
      arguments: [ti.resourceUri],
    };
    return ti;
  }
  async getChildren(
    element?: TeleportTreeItem | undefined
  ): Promise<TeleportTreeItem[]> {
    const uri = element?.uri ?? Uri.from({ scheme, path: '/' });
    const dir = await workspace.fs.readDirectory(uri);

    return dir.map(([name, fileType]) => ({
      uri: Uri.joinPath(uri, name),
      fileType,
    }));
  }
}
