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
import type { DbMeta } from 'teleport/Discover/useDiscover';

export function useCreateDatabase(props: AgentStepProps) {
  const ctx = useTeleport();
  const { attempt, setAttempt } = useAttempt('');

  async function createDbAndQueryDb(db: CreateDatabaseRequest) {
    setAttempt({ status: 'processing' });
    try {
      // TODO (lisa): The exisitng logic below is no longer correct, will modify/update
      // after this issue gets resolved: https://github.com/gravitational/teleport/issues/19032
      //
      // Logic to implement:
      //
      // 1) See if there is a service/agent that can pick up this database (matching labels)
      //    Note: since defining labels in this step is optional,
      //          only an agent that has asteriks in its labels can pick it up
      // 2) Based on whether service exists:
      //    - If exists:
      //      - create database
      //      - wait for it to be picked up by the existing service
      //      - skip next step (take user directly to set up mutual TLS)
      //    - If not exists:
      //      - create database
      //      - take user to next step that instructs user to add a service
      //        ** save the labels user defined in here, and set it as the default
      //           for the next step (this is how the agent will pick up the db)
      //        ** if user did not define any labels, then next step will require asteriks

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

      const dbMeta: DbMeta = {
        ...(props.agentMeta as DbMeta),
        resourceName: db.name,
        agentMatcherLabels: db.labels,
      };

      // If an agent was found, skip the next step that requires you
      // to set up the db service, and set the database we queried to
      // refer to it in later steps (this queried db will include current
      // db users and db names).
      const queriedDb = queryResult.agents[0];
      if (queriedDb) {
        numSteps = 2;
        props.updateAgentMeta({
          ...dbMeta,
          db: queriedDb,
        });
      } else {
        // Set the new db name to query by this name after user
        // adds a db service.
        props.updateAgentMeta(dbMeta);
      }
      props.nextStep(numSteps);
    } catch (err) {
      let message;
      if (err instanceof Error) message = err.message;
      else message = String(err);
      setAttempt({ status: 'failed', statusText: message });
    }
  }

  const access = ctx.storeUser.getDatabaseAccess();
  return {
    attempt,
    createDbAndQueryDb,
    canCreateDatabase: access.create,
  };
}

export type State = ReturnType<typeof useCreateDatabase>;
