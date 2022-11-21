/**
 * Copyright 2022 Gravitational, Inc.
 *
 * Licensed under the Apache License, Version 2.0 (the "License");
 * you may not use this file except in compliance with the License.
 * You may obtain a copy of the License at
 *
 *     http://www.apache.org/licenses/LICENSE-2.0
 *
 * Unless required by applicable law or agreed to in writing, software
 * distributed under the License is distributed on an "AS IS" BASIS,
 * WITHOUT WARRANTIES OR CONDITIONS OF ANY KIND, either express or implied.
 * See the License for the specific language governing permissions and
 * limitations under the License.
 */

import useAttempt from 'shared/hooks/useAttemptNext';

import useTeleport from 'teleport/useTeleport';

import type { AgentStepProps } from '../../types';
import type { CreateDatabaseRequest } from 'teleport/services/databases';

export function useCreateDatabase(props: AgentStepProps) {
  const ctx = useTeleport();
  const { attempt, setAttempt } = useAttempt('processing');

  async function createDbAndQueryDb(db: CreateDatabaseRequest) {
    setAttempt({ status: 'processing' });
    try {
      const clusterId = ctx.storeUser.getClusterId();
      // Create the Database.
      await ctx.databaseService.createDatabase(clusterId, db);

      // Query for the created database by searching through database services.
      // As discussed, if a service was already available, the new db should be
      // picked up immediately.
      let numSteps;
      const request = {
        search: db.name,
        limit: 1,
      };
      const queryResult = await ctx.databaseService.fetchDatabases(
        clusterId,
        request
      );

      // If an agent was found, skip the next step that requires you
      // to set up the db service, and set the database we queried to
      // refer to it in later steps (this queried db will include current
      // db users and db names).
      const queriedDb = queryResult.agents[0];
      if (queriedDb) {
        numSteps = 2;
        props.updateAgentMeta({
          ...props.agentMeta,
          resourceName: queriedDb.name,
          db: queriedDb,
        });
      }
      props.nextStep(numSteps);
    } catch (err) {
      let message;
      if (err instanceof Error) message = err.message;
      else message = String(err);
      setAttempt({ status: 'failed', statusText: message });
    }
  }

  return {
    attempt,
    createDbAndQueryDb,
  };
}

export type State = ReturnType<typeof useCreateDatabase>;
