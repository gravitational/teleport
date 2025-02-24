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

import { Report } from 'gen-proto-ts/teleport/lib/vnet/diag/v1/diag_pb';

import type { Shell } from 'teleterm/mainProcess/shell';
import type { RuntimeSettings } from 'teleterm/mainProcess/types';
import * as uri from 'teleterm/ui/uri';
import {
  DocumentUri,
  KubeUri,
  paths,
  RootClusterUri,
  routing,
  ServerUri,
} from 'teleterm/ui/uri';
import { unique } from 'teleterm/ui/utils/uid';

import { getDocumentGatewayTitle } from './documentsUtils';
import {
  CreateAccessRequestDocumentOpts,
  CreateGatewayDocumentOpts,
  CreateTshKubeDocumentOptions,
  Document,
  DocumentAccessRequests,
  DocumentAuthorizeWebSession,
  DocumentCluster,
  DocumentClusterQueryParams,
  DocumentConnectMyComputer,
  DocumentGateway,
  DocumentGatewayCliClient,
  DocumentGatewayKube,
  DocumentOrigin,
  DocumentPtySession,
  DocumentTshKube,
  DocumentTshNode,
  DocumentTshNodeWithServerId,
  DocumentVnetDiagReport,
  WebSessionRequest,
} from './types';

export class DocumentsService {
  constructor(
    private getState: () => { documents: Document[]; location: string },
    private setState: (
      draftState: (draft: { documents: Document[]; location: string }) => void
    ) => void
  ) {}

  open(docUri: DocumentUri) {
    if (!this.getDocument(docUri)) {
      this.add({
        uri: docUri,
        title: docUri,
        kind: 'doc.blank',
      });
    }

    this.setLocation(docUri);
  }

  createAccessRequestDocument(
    opts: CreateAccessRequestDocumentOpts
  ): DocumentAccessRequests {
    const uri = routing.getDocUri({ docId: unique() });
    let title: string;
    switch (opts.state) {
      case 'creating':
        title = 'New Role Request';
        break;
      case 'reviewing':
        title = `Access Request: ${opts.requestId.slice(-5)}`;
        break;
      case 'browsing':
      default:
        title = 'Access Requests';
    }
    return {
      uri,
      clusterUri: opts.clusterUri,
      requestId: opts.requestId,
      title,
      kind: 'doc.access_requests',
      state: opts.state,
    };
  }

  /** @deprecated Use createClusterDocument function instead of the method on DocumentsService. */
  createClusterDocument(opts: {
    clusterUri: uri.ClusterUri;
    queryParams?: DocumentClusterQueryParams;
  }): DocumentCluster {
    return createClusterDocument(opts);
  }

  /**
   * @deprecated Use createGatewayKubeDocument instead.
   * DELETE IN 15.0.0. See DocumentGatewayKube for more details.
   */
  createTshKubeDocument(
    options: CreateTshKubeDocumentOptions
  ): DocumentTshKube {
    const { params } = routing.parseKubeUri(options.kubeUri);
    const uri = routing.getDocUri({ docId: unique() });
    return {
      uri,
      kind: 'doc.terminal_tsh_kube',
      status: 'connecting',
      rootClusterId: params.rootClusterId,
      leafClusterId: params.leafClusterId,
      kubeId: params.kubeId,
      kubeUri: options.kubeUri,
      // We prepend the name with `rootClusterId/` to create a kube config
      // inside this directory. When the user logs out of the cluster,
      // the entire directory is deleted.
      kubeConfigRelativePath:
        options.kubeConfigRelativePath ||
        `${params.rootClusterId}/${params.kubeId}-${unique(5)}`,
      title: params.kubeId,
      origin: options.origin,
    };
  }

  createTshNodeDocument(
    serverUri: ServerUri,
    params: { origin: DocumentOrigin }
  ): DocumentTshNodeWithServerId {
    const { params: routingParams } = routing.parseServerUri(serverUri);
    const uri = routing.getDocUri({ docId: unique() });

    return {
      uri,
      kind: 'doc.terminal_tsh_node',
      status: 'connecting',
      rootClusterId: routingParams.rootClusterId,
      leafClusterId: routingParams.leafClusterId,
      serverId: routingParams.serverId,
      serverUri,
      title: '',
      login: '',
      origin: params.origin,
    };
  }

  /**
   * If title is not present in opts, createGatewayDocument will create one based on opts.
   */
  createGatewayDocument(opts: CreateGatewayDocumentOpts): DocumentGateway {
    const {
      targetUri,
      targetUser,
      targetName,
      targetSubresourceName,
      port,
      gatewayUri,
      origin,
    } = opts;
    const uri = routing.getDocUri({ docId: unique() });

    const doc: DocumentGateway = {
      uri,
      kind: 'doc.gateway',
      targetUri,
      targetUser,
      targetName,
      targetSubresourceName,
      gatewayUri,
      title: undefined,
      port,
      origin,
      status: '',
    };
    doc.title = getDocumentGatewayTitle(doc);
    return doc;
  }

  createGatewayCliDocument({
    title,
    targetUri,
    targetUser,
    targetName,
    targetProtocol,
  }: Pick<
    DocumentGatewayCliClient,
    'title' | 'targetUri' | 'targetUser' | 'targetName' | 'targetProtocol'
  >): DocumentGatewayCliClient {
    const clusterUri = routing.ensureClusterUri(targetUri);
    const { rootClusterId, leafClusterId } =
      routing.parseClusterUri(clusterUri).params;

    return {
      kind: 'doc.gateway_cli_client',
      uri: routing.getDocUri({ docId: unique() }),
      title,
      status: 'connecting',
      rootClusterId,
      leafClusterId,
      targetUri,
      targetUser,
      targetName,
      targetProtocol,
    };
  }

  createGatewayKubeDocument({
    targetUri,
    origin,
  }: {
    targetUri: KubeUri;
    origin: DocumentOrigin;
  }): DocumentGatewayKube {
    const uri = routing.getDocUri({ docId: unique() });
    const { params } = routing.parseKubeUri(targetUri);

    return {
      uri,
      kind: 'doc.gateway_kube',
      rootClusterId: params.rootClusterId,
      leafClusterId: params.leafClusterId,
      targetUri,
      title: `${params.kubeId}`,
      origin,
      status: '',
    };
  }

  createAuthorizeWebSessionDocument(params: {
    rootClusterUri: string;
    webSessionRequest: WebSessionRequest;
  }): DocumentAuthorizeWebSession {
    const uri = routing.getDocUri({ docId: unique() });

    return {
      uri,
      kind: 'doc.authorize_web_session',
      title: 'Authorize Web Session',
      rootClusterUri: params.rootClusterUri,
      webSessionRequest: params.webSessionRequest,
    };
  }

  createVnetDiagReportDocument(opts: {
    rootClusterUri: RootClusterUri;
    report: Report;
  }): DocumentVnetDiagReport {
    const uri = routing.getDocUri({ docId: unique() });

    return {
      uri,
      kind: 'doc.vnet_diag_report',
      title: 'VNet Diagnostic Report',
      rootClusterUri: opts.rootClusterUri,
      report: opts.report,
    };
  }

  openConnectMyComputerDocument(opts: {
    // URI of the root cluster could be passed to the `DocumentsService`
    // constructor and then to the document, instead of being taken from the parameter.
    // However, we decided not to do so because other documents are based only on the provided parameters.
    rootClusterUri: RootClusterUri;
  }): void {
    const existingDoc = this.findFirstOfKind('doc.connect_my_computer');
    if (existingDoc) {
      this.open(existingDoc.uri);
      return;
    }

    const doc: DocumentConnectMyComputer = {
      uri: routing.getDocUri({ docId: unique() }),
      kind: 'doc.connect_my_computer' as const,
      title: 'Connect My Computer',
      rootClusterUri: opts.rootClusterUri,
      status: '',
    };
    this.add(doc);
    this.open(doc.uri);
  }

  openNewTerminal(opts: { rootClusterId: string; leafClusterId?: string }) {
    const doc = ((): Document => {
      const activeDocument = this.getActive();

      if (activeDocument && activeDocument.kind == 'doc.terminal_shell') {
        // Copy activeDocument to use the same cwd in the new doc.
        return {
          ...activeDocument,
          uri: routing.getDocUri({ docId: unique() }),
          // Do not inherit the shell of this document when opening a new one, use default.
          shellId: undefined,
          ...opts,
        };
      } else {
        return {
          uri: routing.getDocUri({ docId: unique() }),
          title: 'Terminal',
          kind: 'doc.terminal_shell',
          ...opts,
        };
      }
    })();

    this.add(doc);
    this.setLocation(doc.uri);
  }

  getDocuments() {
    return this.getState().documents;
  }

  getDocument(uri: string) {
    return this.getState().documents.find(i => i.uri === uri);
  }

  findFirstOfKind(documentKind: Document['kind']): Document | undefined {
    return this.getState().documents.find(d => d.kind === documentKind);
  }

  getActive() {
    return this.getDocument(this.getLocation());
  }

  getLocation() {
    return this.getState().location;
  }

  duplicatePtyAndActivate(uri: string) {
    const documentIndex = this.getState().documents.findIndex(
      d => d.uri === uri
    );
    const newDocument = {
      ...this.getState().documents[documentIndex],
      uri: routing.getDocUri({ docId: unique() }),
    };
    this.add(newDocument, documentIndex + 1);
    this.setLocation(newDocument.uri);
  }

  close(uri: string) {
    if (uri === paths.docHome) {
      return;
    }

    this.setState(draft => {
      if (draft.location === uri) {
        draft.location = this.getNextUri(uri);
      }

      draft.documents = this.getState().documents.filter(d => d.uri !== uri);
    });
  }

  closeOthers(uri: string) {
    this.filter(uri).forEach(d => this.close(d.uri));
  }

  closeToRight(uri: string) {
    const documentIndex = this.getState().documents.findIndex(
      d => d.uri === uri
    );
    this.getState()
      .documents.filter((_, index) => index > documentIndex)
      .forEach(d => this.close(d.uri));
  }

  isActive(uri: string) {
    const location = this.getLocation();
    return !!routing.parseUri(location, {
      exact: true,
      path: uri,
    });
  }

  add(doc: Document, position?: number) {
    this.setState(draft => {
      if (position === undefined) {
        draft.documents.push(doc);
      } else {
        draft.documents.splice(position, 0, doc);
      }
    });
  }

  /**
   * Updates the document by URI.
   * @param uri - document URI.
   * @param updated - a new document object or an update function.
   */
  update(
    uri: DocumentUri,
    updated: Partial<Document> | ((draft: Document) => void)
  ) {
    this.setState(draft => {
      const toUpdate = draft.documents.find(doc => doc.uri === uri);
      if (typeof updated === 'function') {
        updated(toUpdate);
      } else {
        Object.assign(toUpdate, updated);
      }
    });
  }

  refreshPtyTitle(
    uri: DocumentUri,
    {
      shell,
      cwd,
      clusterName,
      runtimeSettings,
    }: {
      shell: Shell;
      cwd: string;
      clusterName: string;
      runtimeSettings: Pick<RuntimeSettings, 'platform' | 'defaultOsShellId'>;
    }
  ): void {
    const doc = this.getDocument(uri);
    if (!doc) {
      throw Error(`Document ${uri} does not exist`);
    }
    const omitShellName =
      (runtimeSettings.platform === 'linux' ||
        runtimeSettings.platform === 'darwin') &&
      shell.id === runtimeSettings.defaultOsShellId;
    const shellBinName = !omitShellName && shell.binName;
    if (doc.kind === 'doc.terminal_shell') {
      this.update(doc.uri, {
        cwd,
        title: [shellBinName, cwd, clusterName].filter(Boolean).join(' · '),
      });
      return;
    }

    if (doc.kind === 'doc.gateway_kube') {
      const { params } = routing.parseKubeUri(doc.targetUri);
      this.update(doc.uri, {
        title: [params.kubeId, shellBinName].filter(Boolean).join(' · '),
      });
    }
  }

  replace(uri: DocumentUri, document: Document): void {
    const documentToCloseIndex = this.getDocuments().findIndex(
      doc => doc.uri === uri
    );
    const documentToClose = this.getDocuments().at(documentToCloseIndex);
    if (documentToClose) {
      this.close(documentToClose.uri);
    }
    this.add(document, documentToClose ? documentToCloseIndex : undefined);
    this.open(document.uri);
  }

  reopenPtyInShell<T extends DocumentPtySession | DocumentGatewayKube>(
    document: T,
    shell: Shell
  ): void {
    // We assign a new URI to render a new document.
    const newDocument: T = { ...document, shellId: shell.id, uri: unique() };
    this.replace(document.uri, newDocument);
  }

  filter(uri: string) {
    return this.getState().documents.filter(i => i.uri !== uri);
  }

  getTshNodeDocuments() {
    function isTshNode(d: DocumentTshNode): d is DocumentTshNode {
      return d.kind === 'doc.terminal_tsh_node';
    }

    return this.getState().documents.filter(isTshNode);
  }

  getGatewayDocuments() {
    function isGw(d: DocumentGateway): d is DocumentGateway {
      return d.kind === 'doc.gateway';
    }

    return this.getState().documents.filter(isGw);
  }

  getNextUri(uri: string) {
    const docs = this.getState().documents;
    for (let i = 0; i < docs.length; i++) {
      if (docs[i].uri === uri) {
        if (docs.length > i + 1) {
          return docs[i + 1].uri;
        }

        if (docs.length === i + 1 && i !== 0) {
          return docs[i - 1].uri;
        }
      }
    }

    return '/';
  }

  findClusterDocument(clusterUri: string) {
    return this.getState().documents.find(
      i => i.kind === 'doc.cluster' && i.clusterUri === clusterUri
    );
  }

  setLocation(location: string) {
    this.setState(draft => {
      draft.location = location;
    });
  }

  swapPosition(oldIndex: number, newIndex: number) {
    this.setState(draft => {
      const doc = draft.documents[oldIndex];
      draft.documents.splice(oldIndex, 1);
      draft.documents.splice(newIndex, 0, doc);
    });
  }
}

export function createClusterDocument(opts: {
  clusterUri: uri.ClusterUri;
  queryParams?: DocumentClusterQueryParams;
}): DocumentCluster {
  const uri = routing.getDocUri({ docId: unique() });
  const clusterName = routing.parseClusterName(opts.clusterUri);
  return {
    uri,
    clusterUri: opts.clusterUri,
    title: clusterName,
    kind: 'doc.cluster',
    queryParams: opts.queryParams || getDefaultDocumentClusterQueryParams(),
  };
}

export function getDefaultDocumentClusterQueryParams(): DocumentClusterQueryParams {
  return {
    resourceKinds: [],
    search: '',
    sort: { fieldName: 'name', dir: 'ASC' },
    advancedSearchEnabled: false,
  };
}
