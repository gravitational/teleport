/**
 * Teleport
 * Copyright (C) 2024 Gravitational, Inc.
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
import {
  nativeImage,
  Tray,
  Menu,
  clipboard,
  MenuItemConstructorOptions,
} from 'electron';

import { Gateway } from 'gen-proto-ts/teleport/lib/teleterm/v1/gateway_pb';

import { getAssetPath } from 'teleterm/mainProcess/runtimeSettings';
import { TshdClient } from 'teleterm/services/tshd';
import { routing } from 'teleterm/ui/uri';
import { maybeUserAtProxyHost } from 'teleterm/services/tshd/cluster';

export function addTray(tshd: TshdClient) {
  const image = nativeImage.createFromPath(getAssetPath('iconTemplate.png'));
  const resizedImage = image.resize({ width: 16 });
  resizedImage.setTemplateImage(true);
  const tray = new Tray(resizedImage);
  const allGateways = [];

  tray.on('mouse-enter', async () => {
    const gatewayMenuItems = await getGatewayMenuItems(tshd, allGateways);
    const profiles = await getProfiles(tshd);

    // TODO: Guarantee that there is only one promise running that updates the menu.
    const contextMenu = Menu.buildFromTemplate([
      {
        label: 'Open Teleport Connect',
        icon: nativeImage
          .createFromNamedImage('NSImageNameApplicationIcon')
          .resize({ width: 16 }),
        type: 'normal',
      },
      profiles,
      { type: 'separator' },
      {
        label: 'Local proxies',
        type: 'normal',
        enabled: false,
      },
      ...gatewayMenuItems,
      { type: 'separator' },
      { label: 'Quit', type: 'normal' },
    ]);
    tray.setContextMenu(contextMenu);
  });
}

async function getGatewayMenuItems(
  tshdClient: TshdClient,
  allGateways: Gateway[]
): Promise<MenuItemConstructorOptions[]> {
  const { response } = await tshdClient.listGateways({});
  const { gateways } = response;
  gateways.forEach(gateway => {
    if (!allGateways.find(allGateway => allGateway.uri === gateway.uri)) {
      allGateways.push(gateway);
    }
  });

  return allGateways.map(g => {
    const address = `${g.localAddress}:${g.localPort}`;
    const isConnected = gateways.find(gateway => g.uri === gateway.uri);
    const turnOn = {
      label: 'Turn on',
      click: async () => {
        const { response: newGateway } = await tshdClient.createGateway(g);
        const index = allGateways.findIndex(
          allGateway => g.uri === allGateway.uri
        );
        // replace the gateway
        if (index >= 0) {
          allGateways[index] = newGateway;
        }
      },
    };
    const turnOff = {
      label: 'Turn off',
      click: () => {
        tshdClient.removeGateway({ gatewayUri: g.uri });
      },
    };
    return {
      label: `${g.targetUser}@${g.targetName} (${routing.parseClusterName(g.targetUri)})`,
      icon: nativeImage
        .createFromNamedImage(
          isConnected ? 'NSImageNameStatusAvailable' : 'NSImageNameStatusNone'
        )
        .resize({ width: 16 }),
      type: 'submenu' as const,
      submenu: [
        {
          label: address,
          type: 'normal',
          enabled: false,
        },
        { type: 'separator' },
        {
          label: 'Copy address',
          type: 'normal',
          click: () => clipboard.writeText(address),
        },
        isConnected ? turnOff : turnOn,
      ],
    };
  });
}

const getProfiles = async (
  tshd: TshdClient
): Promise<MenuItemConstructorOptions> => {
  const {
    response: { clusters: rootClusters, currentRootClusterUri },
  } = await tshd.listRootClusters({});
  const currentCluster = rootClusters.find(
    c => c.uri === currentRootClusterUri
  );

  return {
    label: maybeUserAtProxyHost(currentCluster),
    icon: nativeImage
      .createFromNamedImage('NSImageNameUser')
      .resize({ width: 16 }),
    type: rootClusters.length > 1 ? 'submenu' : 'normal',
    submenu:
      rootClusters.length > 1
        ? rootClusters.map(c => ({
            label: maybeUserAtProxyHost(c),
            type: 'radio',
            checked: c.uri === currentRootClusterUri,
          }))
        : undefined,
  };
};
