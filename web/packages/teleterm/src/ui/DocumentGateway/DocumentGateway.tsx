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

import React from 'react';

import Document from 'teleterm/ui/Document';
import * as types from 'teleterm/ui/services/workspacesService';

import { useAppContext } from 'teleterm/ui/appContextProvider';

import { OfflineDocumentGateway } from './Offline/OfflineDocumentGateway';
import { OnlineDocumentGateway } from './Online/OnlineDocumentGateway';

type Props = {
  visible: boolean;
  doc: types.DocumentGateway;
};

export function DocumentGateway(props: Props) {
  const ctx = useAppContext();
  const gateway = ctx.clustersService.findGateway(props.doc.gatewayUri);

  if (!gateway) {
    return (
      <Document visible={props.visible}>
        <OfflineDocumentGateway doc={props.doc} />
      </Document>
    );
  }

  return (
    <Document visible={props.visible}>
      <OnlineDocumentGateway doc={props.doc} gateway={gateway} />
    </Document>
  );
}
