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

import { useAppContext } from 'teleterm/ui/appContextProvider';

export default function useServerConnect({ serverUri, onClose }: Props) {
  const ctx = useAppContext();
  const server = ctx.clustersService.getServer(serverUri);
  const cluster = ctx.clustersService.findClusterByResource(serverUri);
  const logins = cluster?.loggedInUser?.sshLoginsList || [];

  const connect = (login: string) => {
    const documentsService = ctx.workspacesService.getWorkspaceDocumentService(
      cluster.uri
    );
    const doc = documentsService.createTshNodeDocument(serverUri);
    doc.title = `${login}@${server.hostname}`;
    doc.login = login;

    documentsService.add(doc);
    documentsService.setLocation(doc.uri);

    onClose();
  };

  return {
    server,
    logins,
    connect,
    onClose,
  };
}

export type Props = {
  onClose(): void;
  serverUri: string;
};

export type State = ReturnType<typeof useServerConnect>;
