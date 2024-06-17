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

import { App } from 'gen-proto-ts/teleport/lib/teleterm/v1/app_pb';

import { routing } from 'teleterm/ui/uri';
import { IAppContext } from 'teleterm/ui/types';

import {
  getWebAppLaunchUrl,
  isWebApp,
  getAwsAppLaunchUrl,
  getSamlAppSsoUrl,
} from 'teleterm/services/tshd/app';

import { DocumentOrigin } from './types';

/**
 * connectToApp launches an app in the browser, with the exception of TCP apps, for which it either
 * sets up an app gateway or launches VNet if supported.
 *
 * Unlike other connectTo* functions, connectToApp is oriented towards the search bar. In other
 * contexts outside of the search bar, you typically want to open apps in the browser. In that case,
 * you don't need connectToApp – you can just use a regular link instead. In the search bar you
 * select a div, so there's no href you can add.
 */
export async function connectToApp(
  ctx: IAppContext,
  /**
   * launchVnet is supposed to be provided if VNet is supported. If so, connectToApp is going to use
   * this function when targeting a TCP app. Otherwise it'll create an app gateway.
   */
  launchVnet: null | (() => Promise<[void, Error]>),
  target: App,
  telemetry: { origin: DocumentOrigin },
  options?: {
    arnForAwsApp?: string;
  }
): Promise<void> {
  const rootClusterUri = routing.ensureRootClusterUri(target.uri);
  const rootCluster = ctx.clustersService.findCluster(rootClusterUri);
  const cluster = ctx.clustersService.findClusterByResource(target.uri);

  if (target.samlApp) {
    launchAppInBrowser(
      ctx,
      target,
      getSamlAppSsoUrl({
        app: target,
        rootCluster,
      }),
      telemetry
    );
    return;
  }

  if (target.awsConsole) {
    launchAppInBrowser(
      ctx,
      target,
      getAwsAppLaunchUrl({
        app: target,
        rootCluster,
        cluster,
        arn: options.arnForAwsApp,
      }),
      telemetry
    );
    return;
  }

  if (target.endpointUri.startsWith('cloud://')) {
    alert('Cloud apps are supported only in tsh.');
    return;
  }

  if (isWebApp(target)) {
    launchAppInBrowser(
      ctx,
      target,
      getWebAppLaunchUrl({
        app: target,
        rootCluster,
        cluster,
      }),
      telemetry
    );
    return;
  }

  // TCP app
  if (launchVnet) {
    await connectToAppWithVnet(ctx, launchVnet, target);
    return;
  }

  await setUpAppGateway(ctx, target, telemetry);
}

export async function setUpAppGateway(
  ctx: IAppContext,
  target: App,
  telemetry: { origin: DocumentOrigin }
) {
  const rootClusterUri = routing.ensureRootClusterUri(target.uri);

  const documentsService =
    ctx.workspacesService.getWorkspaceDocumentService(rootClusterUri);
  const doc = documentsService.createGatewayDocument({
    targetUri: target.uri,
    origin: telemetry.origin,
    targetName: routing.parseAppUri(target.uri).params.appId,
    targetUser: '',
  });

  const connectionToReuse = ctx.connectionTracker.findConnectionByDocument(doc);

  if (connectionToReuse) {
    await ctx.connectionTracker.activateItem(connectionToReuse.id, {
      origin: telemetry.origin,
    });
  } else {
    await ctx.workspacesService.setActiveWorkspace(rootClusterUri);
    documentsService.add(doc);
    documentsService.open(doc.uri);
  }
}

export async function connectToAppWithVnet(
  ctx: IAppContext,
  launchVnet: () => Promise<[void, Error]>,
  target: App
) {
  const [, err] = await launchVnet();
  if (err) {
    return;
  }

  const addrToCopy = target.publicAddr;
  try {
    await navigator.clipboard.writeText(addrToCopy);
  } catch (error) {
    // On macOS, if the user uses the mouse rather than the keyboard to proceed with the osascript
    // prompt, the Electron app throws an error about clipboard write permission being denied.
    if (error['name'] === 'NotAllowedError') {
      console.error(error);
      ctx.notificationsService.notifyInfo(
        `Connect via VNet by using ${addrToCopy}.`
      );
      return;
    }
    throw error;
  }
  ctx.notificationsService.notifyInfo(
    `Connect via VNet by using ${addrToCopy} (copied to clipboard).`
  );
}

/**
 * When the app is opened outside Connect,
 * the usage event has to be captured manually.
 */
export function captureAppLaunchInBrowser(
  ctx: IAppContext,
  target: Pick<App, 'uri'>,
  telemetry: { origin: DocumentOrigin }
) {
  ctx.usageService.captureProtocolUse({
    uri: target.uri,
    protocol: 'app',
    origin: telemetry.origin,
    accessThrough: 'proxy_service',
  });
}

function launchAppInBrowser(
  ctx: IAppContext,
  target: Pick<App, 'uri'>,
  launchUrl: string,
  telemetry: { origin: DocumentOrigin }
) {
  captureAppLaunchInBrowser(ctx, target, telemetry);

  // Generally, links should be opened with <a> elements.
  // Unfortunately, in some cases it is not possible,
  // for example, in the search bar.
  window.open(launchUrl, '_blank', 'noreferrer,noopener');
}
