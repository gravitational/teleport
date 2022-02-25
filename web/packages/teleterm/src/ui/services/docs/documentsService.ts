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

import { useStore } from 'shared/libs/stores';
import { unique } from 'teleterm/ui/utils/uid';
import {
  DocumentTshKube,
  Document,
  DocumentGateway,
  DocumentTshNode,
  CreateGatewayDocumentOpts,
  CreateClusterDocumentOpts,
  DocumentCluster,
} from './types';
import { ImmutableStore } from '../immutableStore';
import { routing, paths } from 'teleterm/ui/uri';

type State = {
  location: string;
  docs: Document[];
};

export class DocumentsService extends ImmutableStore<State> {
  state: State = {
    location: paths.docHome,
    docs: [
      {
        uri: paths.docHome,
        kind: 'doc.home',
        title: 'Home',
      },
    ],
  };

  constructor() {
    super();
  }

  open(docUri: string) {
    if (!this.getDocument(docUri)) {
      this.add({
        uri: docUri,
        title: docUri,
        kind: 'doc.blank',
      });
    }

    this.setLocation(docUri);
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

  createTshKubeDocument(kubeUri: string): DocumentTshKube {
    const { params } = routing.parseKubeUri(kubeUri);
    const uri = routing.getDocUri({ docId: unique() });
    return {
      uri,
      kind: 'doc.terminal_tsh_kube',
      status: 'connecting',
      rootClusterId: params.rootClusterId,
      leafClusterId: params.leafClusterId,
      kubeId: params.kubeId,
      kubeUri,
      title: params.kubeId,
    };
  }

  createTshNodeDocument(serverUri: string): DocumentTshNode {
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

  createGatewayDocument(opts: CreateGatewayDocumentOpts): DocumentGateway {
    const { gatewayUri, targetUri, title, targetUser, port } = opts;
    const uri = routing.getDocUri({ docId: unique() });
    return {
      uri,
      kind: 'doc.gateway',
      gatewayUri,
      targetUri,
      targetUser,
      title,
      port,
    };
  }

  openNewTerminal() {
    const doc = ((): Document => {
      const activeDocument = this.getActive();
      switch (activeDocument.kind) {
        case 'doc.terminal_shell':
          return {
            ...activeDocument,
            uri: routing.getDocUri({ docId: unique() }),
          };
        default:
          return {
            uri: routing.getDocUri({ docId: unique() }),
            title: 'Terminal',
            kind: 'doc.terminal_shell',
          };
      }
    })();

    this.add(doc);
    this.setLocation(doc.uri);
  }

  getDocuments() {
    return this.state.docs;
  }

  getDocument(uri: string) {
    return this.state.docs.find(i => i.uri === uri);
  }

  getActive() {
    return this.getDocument(this.getLocation());
  }

  getLocation() {
    return this.state.location;
  }

  duplicatePtyAndActivate(uri: string) {
    const documentIndex = this.state.docs.findIndex(d => d.uri === uri);
    const newDocument = {
      ...this.state.docs[documentIndex],
      uri: routing.getDocUri({ docId: unique() }),
    };
    this.add(newDocument, documentIndex + 1);
    this.setLocation(newDocument.uri);
  }

  close(uri: string) {
    if (uri === paths.docHome) {
      return;
    }

    const nextUri = this.getNextUri(uri);
    const docs = this.state.docs.filter(d => d.uri !== uri);
    this.setState(draft => ({ ...draft, docs, location: nextUri }));
  }

  closeOthers(uri: string) {
    this.filter(uri).forEach(d => this.close(d.uri));
  }

  closeToRight(uri: string) {
    const documentIndex = this.state.docs.findIndex(d => d.uri === uri);
    this.state.docs
      .filter((_, index) => index > documentIndex)
      .forEach(d => this.close(d.uri));
  }

  isActive(uri: string) {
    const location = this.getLocation();
    return !!routing.parseUri(location, { exact: true, path: uri });
  }

  isClusterDocumentActive(clusterUri: string) {
    const doc = this.getActive();
    return doc.kind === 'doc.cluster' && doc.clusterUri === clusterUri;
  }

  add(doc: Document, position?: number) {
    this.setState(draft => {
      if (position === undefined) {
        draft.docs.push(doc);
      } else {
        draft.docs.splice(position, 0, doc);
      }
    });
  }

  update(uri: string, partialDoc: Partial<Document>) {
    this.setState(draft => {
      const toUpdate = draft.docs.find(doc => doc.uri === uri);
      Object.assign(toUpdate, partialDoc);
    });
  }

  filter(uri: string) {
    return this.state.docs.filter(i => i.uri !== uri);
  }

  getTshNodeDocuments() {
    function isTshNode(d: DocumentTshNode): d is DocumentTshNode {
      return d.kind === 'doc.terminal_tsh_node';
    }

    return this.state.docs.filter(isTshNode);
  }

  getGatewayDocuments() {
    function isGw(d: DocumentGateway): d is DocumentGateway {
      return d.kind === 'doc.gateway';
    }

    return this.state.docs.filter(isGw);
  }

  getNextUri(uri: string) {
    const docs = this.state.docs;
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
    return this.state.docs.find(
      i => i.kind === 'doc.cluster' && i.clusterUri === clusterUri
    );
  }

  setLocation(location: string) {
    this.setState(draft => {
      draft.location = location;
    });
  }

  useState() {
    return useStore(this).state;
  }

  swapPosition(oldIndex: number, newIndex: number) {
    // account for hidden "home" document
    // TODO(alex-kovoy): consider removing "home" document from the service
    if (this.state.docs.some(d => d.kind === 'doc.home')) {
      oldIndex += 1;
      newIndex += 1;
    }

    const doc = this.state.docs[oldIndex];
    this.setState(draft => {
      draft.docs.splice(oldIndex, 1);
      draft.docs.splice(newIndex, 0, doc);
    });
  }
}
