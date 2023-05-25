/**
 * Copyright 2023 Gravitational, Inc
 *
 * Licensed under the Apache License, Version 2.0 (the "License");
 * you may not use this file except in compliance with the License.
 * You may obtain a copy of the License at
 *
 *      http://www.apache.org/licenses/LICENSE-2.0
 *
 * Unless required by applicable law or agreed to in writing, software
 * distributed under the License is distributed on an "AS IS" BASIS,
 * WITHOUT WARRANTIES OR CONDITIONS OF ANY KIND, either express or implied.
 * See the License for the specific language governing permissions and
 * limitations under the License.
 */

import { KubeUri, routing } from 'teleterm/ui/uri';
import { IAppContext } from 'teleterm/ui/types';
import { TrackedKubeConnection } from 'teleterm/ui/services/connectionTracker';

import { DocumentOrigin } from './types';

export async function connectToKube(
  ctx: IAppContext,
  target: { uri: KubeUri },
  telemetry: { origin: DocumentOrigin }
): Promise<void> {
  const rootClusterUri = routing.ensureRootClusterUri(target.uri);
  const documentsService =
    ctx.workspacesService.getWorkspaceDocumentService(rootClusterUri);
  const kubeDoc = documentsService.createTshKubeDocument({
    kubeUri: target.uri,
    origin: telemetry.origin,
  });
  const connection = ctx.connectionTracker.findConnectionByDocument(
    kubeDoc
  ) as TrackedKubeConnection;

  await ctx.workspacesService.setActiveWorkspace(rootClusterUri);

  documentsService.add({
    ...kubeDoc,
    kubeConfigRelativePath:
      connection?.kubeConfigRelativePath || kubeDoc.kubeConfigRelativePath,
  });
  documentsService.open(kubeDoc.uri);
}
