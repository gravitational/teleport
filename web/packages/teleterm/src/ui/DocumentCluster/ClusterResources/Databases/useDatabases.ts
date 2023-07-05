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
import { Database, ServerSideParams } from 'teleterm/services/tshd/types';
import { makeDatabase } from 'teleterm/ui/services/clusters';
import { connectToDatabase } from 'teleterm/ui/services/workspacesService';

import { useServerSideResources } from '../useServerSideResources';

export function useDatabases() {
  const appContext = useAppContext();

  const { fetchAttempt, ...serverSideResources } =
    useServerSideResources<Database>(
      { fieldName: 'name', dir: 'ASC' }, // default sort
      (params: ServerSideParams) =>
        appContext.resourcesService.fetchDatabases(params)
    );

  function connect(db: ReturnType<typeof makeDatabase>, dbUser: string): void {
    const { uri, name, protocol } = db;
    connectToDatabase(
      appContext,
      { uri, name, protocol, dbUser },
      { origin: 'resource_table' }
    );
  }

  return {
    fetchAttempt,
    connect,
    ...serverSideResources,
  };
}

export type State = ReturnType<typeof useDatabases>;
