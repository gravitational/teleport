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
import { useDiscover } from 'teleport/Discover/useDiscover';
// import { usePoll } from 'teleport/Discover/Shared/usePoll';

import { Database } from '../resources';

import type { AgentStepProps } from '../../types';
import type {
  CreateDatabaseRequest,
  // Database as DatabaseResource,
  DatabaseService,
} from 'teleport/services/databases';
import type { AgentLabel } from 'teleport/services/agents';
import type { DbMeta } from 'teleport/Discover/useDiscover';

export const WAITING_TIMEOUT = 30000; // 30 seconds

export function useCreateDatabase(props: AgentStepProps) {
  const ctx = useTeleport();
  const clusterId = ctx.storeUser.getClusterId();
  const { attempt, setAttempt } = useAttempt('');
  const { emitErrorEvent } = useDiscover();

  // const [pollTimeout, setPollTimeout] = useState(0);
  // const [pollActive, setPollActive] = useState(false);

  // Required persisted states to determine if we can skip a request
  // because there can be multiple failed points:
  //  1) failed to create database (network, duplicate name, incorrect format etc)
  //  2) failed to fetch db services (probably mostly network issues)
  //  3) failed to query if there was a matching database service:
  //    - timed out due to combined previous requests taking longer than WAITING_TIMEOUT
  //    - timed out due to failure to query (this would most likely be some kind of
  //      backend error or network failure)
  // const [newDb, setNewDb] = useState<CreateDatabaseRequest>();

  // const { timedOut, result } = usePoll<DatabaseResource>(
  //   signal => fetchDatabaseServer(signal),
  //   pollTimeout,
  //   pollActive,
  //   3000 // interval: poll every 3 seconds
  // );

  // // Handles polling timeout.
  // useEffect(() => {
  //   if (pollActive && Date.now() > pollTimeout) {
  //     setPollActive(false);
  //     setAttempt({
  //       status: 'failed',
  //       statusText:
  //         'Teleport could not detect your new database in time. Please try again.',
  //     });
  //   }
  // }, [pollActive, pollTimeout, timedOut]);

  // // Handles when polling successfully gets
  // // a response.
  // useEffect(() => {
  //   if (!result) return;

  //   setPollTimeout(null);
  //   setPollActive(false);

  //   const numStepsToSkip = 2;
  //   props.updateAgentMeta({
  //     ...(props.agentMeta as DbMeta),
  //     resourceName: newDb.name,
  //     agentMatcherLabels: newDb.labels,
  //     db: result,
  //   });

  //   props.nextStep(numStepsToSkip);
  // }, [result]);

  // function fetchDatabaseServer(signal: AbortSignal) {
  //   const request = {
  //     search: newDb.name,
  //     limit: 1,
  //   };
  //   return ctx.databaseService
  //     .fetchDatabases(clusterId, request, signal)
  //     .then(res => {
  //       if (res.agents.length) {
  //         return res.agents[0];
  //       }
  //       return null;
  //     });
  // }

  async function registerDatabase(db: CreateDatabaseRequest) {
    // // Set the timeout now, because this entire registering process
    // // should take less than WAITING_TIMEOUT.
    // setPollTimeout(Date.now() + WAITING_TIMEOUT);
    // setAttempt({ status: 'processing' });

    // Attempt creating a new Database resource.
    // Handles a case where if there was a later failure point
    // and user decides to change the database fields, a new database
    // is created (ONLY if the database name has changed since this
    // request operation is only a CREATE operation).
    // if (!newDb || db.name != newDb.name) {
    try {
      const createdDb = await ctx.databaseService
        .createDatabase(clusterId, db)
        .catch((error: Error) => {
          emitErrorEvent(error.message);
          throw error;
        });
      // setNewDb(db);
      props.updateAgentMeta({
        ...(props.agentMeta as DbMeta),
        resourceName: db.name,
        agentMatcherLabels: db.labels,
        db: createdDb,
      });
      props.nextStep();
      return;
    } catch (err) {
      handleRequestError(err);
      return;
    }
    // }

    // TODO(lisa): temporary see if we can query this database.
    // try {
    //   const { services } = await ctx.databaseService.fetchDatabaseServices(
    //     clusterId
    //   );

    //   if (!findActiveDatabaseSvc(db.labels, services)) {
    //     props.updateAgentMeta({
    //       ...(props.agentMeta as DbMeta),
    //       resourceName: db.name,
    //       agentMatcherLabels: db.labels,
    //     });
    //     props.nextStep();
    //     return;
    //   }
    // } catch (err) {
    //   handleRequestError(err);
    //   return;
    // }

    // // See if this new database can be picked up by an existing
    // // database service. If there is no active database service,
    // // user is led to the next step.
    // try {
    //   const { services } = await ctx.databaseService.fetchDatabaseServices(
    //     clusterId
    //   );

    //   if (!findActiveDatabaseSvc(db.labels, services)) {
    //     props.updateAgentMeta({
    //       ...(props.agentMeta as DbMeta),
    //       resourceName: db.name,
    //       agentMatcherLabels: db.labels,
    //     });
    //     props.nextStep();
    //     return;
    //   }
    // } catch (err) {
    //   handleRequestError(err);
    //   return;
    // }

    // // Start polling until new database is picked up by an
    // // existing database service.
    // setPollActive(true);
  }

  function clearAttempt() {
    setAttempt({ status: '' });
  }

  function handleRequestError(err) {
    let message;
    if (err instanceof Error) message = err.message;
    else message = String(err);
    setAttempt({ status: 'failed', statusText: message });
  }

  const access = ctx.storeUser.getDatabaseAccess();
  const dbState = props.resourceState as Database;
  return {
    attempt,
    clearAttempt,
    registerDatabase,
    canCreateDatabase: access.create,
    // pollTimeout,
    dbEngine: dbState.engine,
    dbLocation: dbState.location,
  };
}

export type State = ReturnType<typeof useCreateDatabase>;

export function findActiveDatabaseSvc(
  newDbLabels: AgentLabel[],
  dbServices: DatabaseService[]
) {
  if (!dbServices.length) {
    return false;
  }

  // Create a map for db labels for easy lookup.
  let dbKeyMap = {};
  let dbValMap = {};

  newDbLabels.forEach(label => {
    dbKeyMap[label.name] = label.value;
    dbValMap[label.value] = label.name;
  });

  // Check if any service contains an asterik for labels.
  // This means the service with asterik
  // can pick up any database despite labels.
  for (let i = 0; i < dbServices.length; i++) {
    for (const [key, vals] of Object.entries(dbServices[i].matcherLabels)) {
      const foundAsterikAsValue = vals.includes('*');
      // Check if this service contains any labels with asteriks,
      // which means this service can pick up any database regardless
      // of labels.
      if (key === '*' && foundAsterikAsValue) {
        return true;
      }

      // If no newDbLabels labels were defined, no need to look for other matches
      // continue to next key.
      if (!newDbLabels.length) {
        continue;
      }

      // Start matching every combination.

      // This means any key is fine, as long as a
      // value matches.
      if (key === '*' && vals.find(val => dbValMap[val])) {
        return true;
      }

      // This means any value is fine, as long as a
      // key matches.
      if (foundAsterikAsValue && dbKeyMap[key]) {
        return true;
      }

      // Match against key and value.
      const dbVal = dbKeyMap[key];
      if (dbVal && vals.find(val => val === dbVal)) {
        return true;
      }
    }
  }

  return false;
}
