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

import { unique } from 'teleterm/ui/utils/uid';
import {
  ClusterUri,
  DocumentUri,
  ServerUri,
  paths,
  routing,
} from 'teleterm/ui/uri';

import {
  CreateAccessRequestDocumentOpts,
  CreateClusterDocumentOpts,
  CreateGatewayDocumentOpts,
  CreateNewTerminalOpts,
  CreateTshKubeDocumentOptions,
  Document,
  DocumentAccessRequests,
  DocumentCluster,
  DocumentGateway,
  DocumentGatewayCliClient,
  DocumentTshKube,
  DocumentTshNode,
  DocumentTshNodeWithLoginHost,
  DocumentTshNodeWithServerId,
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
    return {
      uri,
      clusterUri: opts.clusterUri,
      requestId: opts.requestId,
      title: opts.title || 'Access Requests',
      kind: 'doc.access_requests',
      state: opts.state,
    };
  }

  createClusterDocument(opts: CreateClusterDocumentOpts): DocumentCluster {
    const uri = routing.getDocUri({ docId: unique() });
    const clusterName = routing.parseClusterName(opts.clusterUri);
    return {
      uri,
      clusterUri: opts.clusterUri,
      title: clusterName,
      kind: 'doc.cluster',
    };
  }

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
    };
  }

  createTshNodeDocument(serverUri: ServerUri): DocumentTshNodeWithServerId {
    const { params } = routing.parseServerUri(serverUri);
    const uri = routing.getDocUri({ docId: unique() });

    return {
      uri,
      kind: 'doc.terminal_tsh_node',
      status: 'connecting',
      rootClusterId: params.rootClusterId,
      leafClusterId: params.leafClusterId,
      serverId: params.serverId,
      serverUri,
      title: '',
      login: '',
    };
  }

  /**
   * createTshNodeDocumentFromLoginHost handles creation of the doc when the server URI is not
   * available, for example when executing `tsh ssh user@host` from the command bar.
   *
   * @param clusterUri - the URI of the cluster which should be used for hostname lookup. That is,
   * the command will succeed only if the given cluster has only a single server with the hostname
   * matching `host`.
   * @param loginHost - the "user@host" pair.
   */
  createTshNodeDocumentFromLoginHost(
    clusterUri: ClusterUri,
    loginHost: string
  ): DocumentTshNodeWithLoginHost {
    const { params } = routing.parseClusterUri(clusterUri);
    const uri = routing.getDocUri({ docId: unique() });

    return {
      uri,
      kind: 'doc.terminal_tsh_node',
      title: loginHost,
      status: 'connecting',
      rootClusterId: params.rootClusterId,
      leafClusterId: params.leafClusterId,
      loginHost,
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
    } = opts;
    const uri = routing.getDocUri({ docId: unique() });
    const title = `${targetUser}@${targetName}`;

    return {
      uri,
      kind: 'doc.gateway',
      targetUri,
      targetUser,
      targetName,
      targetSubresourceName,
      gatewayUri,
      title,
      port,
    };
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

  openNewTerminal(opts: CreateNewTerminalOpts) {
    const doc = ((): Document => {
      const activeDocument = this.getActive();

      if (activeDocument && activeDocument.kind == 'doc.terminal_shell') {
        // Copy activeDocument to use the same cwd in the new doc.
        return {
          ...activeDocument,
          uri: routing.getDocUri({ docId: unique() }),
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
    return !!routing.parseUri(location, { exact: true, path: uri });
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

  update(uri: string, partialDoc: Partial<Document>) {
    this.setState(draft => {
      const toUpdate = draft.documents.find(doc => doc.uri === uri);
      Object.assign(toUpdate, partialDoc);
    });
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
