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
import { useEffect, useState } from 'react';

import useAttempt from 'shared/hooks/useAttemptNext';

import useTeleport from 'teleport/useTeleport';
import { useDiscover } from 'teleport/Discover/useDiscover';
import { usePoll } from 'teleport/Discover/Shared/usePoll';
import { compareByString } from 'teleport/lib/util';

import { Database } from '../resources';

import type { AgentStepProps } from '../../types';
import type {
  CreateDatabaseRequest,
  Database as DatabaseResource,
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

  // isDbCreateErr is a flag that indicates
  // attempt failed from trying to create a database.
  const [isDbCreateErr, setIsDbCreateErr] = useState(false);

  const [pollTimeout, setPollTimeout] = useState(0);
  const [pollActive, setPollActive] = useState(false);

  // Required persisted states to determine if we can skip a request
  // because there can be multiple failed points:
  //  1) failed to create database (network, duplicate name, incorrect format etc)
  //  2) failed to fetch db services (probably mostly network issues)
  //  3) failed to query if there was a matching database service:
  //    - timed out due to combined previous requests taking longer than WAITING_TIMEOUT
  //    - timed out due to failure to query (this would most likely be some kind of
  //      backend error or network failure)
  const [createdDb, setCreatedDb] = useState<CreateDatabaseRequest>();

  const { timedOut, result } = usePoll<DatabaseResource>(
    signal => fetchDatabaseServer(signal),
    pollTimeout,
    pollActive,
    3000 // interval: poll every 3 seconds
  );

  // Handles polling timeout.
  useEffect(() => {
    if (pollActive && Date.now() > pollTimeout) {
      setPollActive(false);
      setAttempt({
        status: 'failed',
        statusText:
          'Teleport could not detect your new database in time. Please try again.',
      });
      // emitErrorEvent(
      //   `timeout polling for new database with an existing service`
      // );
    }
  }, [pollActive, pollTimeout, timedOut]);

  // Handles when polling successfully gets
  // a response.
  useEffect(() => {
    if (!result) return;

    setPollTimeout(null);
    setPollActive(false);

    const numStepsToSkip = 2;
    props.updateAgentMeta({
      ...(props.agentMeta as DbMeta),
      resourceName: createdDb.name,
      agentMatcherLabels: createdDb.labels,
      db: result,
    });

    props.nextStep(numStepsToSkip);
  }, [result]);

  function fetchDatabaseServer(signal: AbortSignal) {
    const request = {
      search: createdDb.name,
      limit: 1,
    };
    return ctx.databaseService
      .fetchDatabases(clusterId, request, signal)
      .then(res => {
        if (res.agents.length) {
          return res.agents[0];
        }
        return null;
      });
  }

  async function registerDatabase(db: CreateDatabaseRequest) {
    // Set the timeout now, because this entire registering process
    // should take less than WAITING_TIMEOUT.
    setPollTimeout(Date.now() + WAITING_TIMEOUT);
    setAttempt({ status: 'processing' });
    setIsDbCreateErr(false);

    // Attempt creating a new Database resource.
    // Handles a case where if there was a later failure point
    // and user decides to change the database fields, a new database
    // is created (ONLY if the database name has changed since this
    // request operation is only a CREATE operation).
    if (!createdDb) {
      try {
        await ctx.databaseService.createDatabase(clusterId, db);
        setCreatedDb(db);
      } catch (err) {
        handleRequestError(err, 'failed to create database: ');
        setIsDbCreateErr(true);
        return;
      }
    }

    function requiresDbUpdate() {
      if (!createdDb) {
        return false;
      }

      if (createdDb.labels.length === db.labels.length) {
        // Sort by label keys.
        const a = createdDb.labels.sort((a, b) =>
          compareByString(a.name, b.name)
        );
        const b = db.labels.sort((a, b) => compareByString(a.name, b.name));

        for (let i = 0; i < a.length; i++) {
          if (JSON.stringify(a[i]) !== JSON.stringify(b[i])) {
            return true;
          }
        }
      }

      return (
        createdDb.uri !== db.uri ||
        createdDb.awsRds?.accountId !== db.awsRds?.accountId ||
        createdDb.awsRds?.resourceId !== db.awsRds?.resourceId
      );
    }

    // Check and see if database resource need to be updated.
    if (requiresDbUpdate()) {
      try {
        await ctx.databaseService.updateDatabase(clusterId, {
          ...db,
        });
        setCreatedDb(db);
      } catch (err) {
        handleRequestError(err, 'failed to update database: ');
        return;
      }
    }

    // See if this new database can be picked up by an existing
    // database service. If there is no active database service,
    // user is led to the next step.
    try {
      const { services } = await ctx.databaseService.fetchDatabaseServices(
        clusterId
      );

      if (!findActiveDatabaseSvc(db.labels, services)) {
        props.updateAgentMeta({
          ...(props.agentMeta as DbMeta),
          resourceName: db.name,
          agentMatcherLabels: db.labels,
        });
        props.nextStep();
        return;
      }
    } catch (err) {
      handleRequestError(err, 'failed to fetch database services: ');
      return;
    }

    // Start polling until new database is picked up by an
    // existing database service.
    setPollActive(true);
  }

  function clearAttempt() {
    setAttempt({ status: '' });
  }

  // eslint-disable-next-line @typescript-eslint/no-unused-vars
  function handleRequestError(err: Error, preErrMsg = '') {
    let message = 'something went wrong';
    if (err instanceof Error) message = err.message;
    setAttempt({ status: 'failed', statusText: message });
    // emitErrorEvent(`${preErrMsg}${message}`);
  }

  const access = ctx.storeUser.getDatabaseAccess();
  const dbState = props.resourceState as Database;
  return {
    attempt,
    clearAttempt,
    registerDatabase,
    canCreateDatabase: access.create,
    pollTimeout,
    dbEngine: dbState.engine,
    dbLocation: dbState.location,
    isDbCreateErr,
  };
}

export type State = ReturnType<typeof useCreateDatabase>;

export function findActiveDatabaseSvc(
  newDbLabels: AgentLabel[],
  dbServices: DatabaseService[]
) {
  if (!dbServices.length) {
    return null;
  }

  // Create maps for the new labels for easy lookup and matching.
  let newDbLabelKeyMap = {};
  let newDbLabelValMap = {};
  let matchedLabelMap = {};

  newDbLabels.forEach(label => {
    newDbLabelKeyMap[label.name] = label.value;
    matchedLabelMap[label.name] = false;
    newDbLabelValMap[label.value] = label.name;
  });

  const newDbHasLabels = newDbLabels.length > 0;
  for (let i = 0; i < dbServices.length; i++) {
    let noMatch = false;

    // Loop through the current service label keys and its value set.
    const currService = dbServices[i];
    // Sorted to have asteriks be the first to test.
    const entries = Object.entries(currService.matcherLabels).sort();
    matchLabels: for (const [key, vals] of entries) {
      // Check if the label contains asteriks, which means this service can
      // pick up any database regardless of other labels.
      const foundAsterikAsValue = vals.includes('*');
      if (key === '*' && foundAsterikAsValue) {
        return currService;
      }

      // If no labels were defined with the new database, then there is no
      // further matching to do with this currService.
      if (!newDbHasLabels) {
        noMatch = true;
        break matchLabels;
      }

      // Start matching by value.
      // Both the service matcher labels and the new db labels must be equal to each other.
      // For example, if the service matcher has other labels that are not defined
      // in the new db labels, then the service cannot detect the new db.

      // This means any key is fine, as long as value matches.
      if (key === '*') {
        let found = false;
        vals.forEach(val => {
          const key = newDbLabelValMap[val];
          if (key) {
            matchedLabelMap[key] = true;
            found = true;
          }
        });
        if (found) {
          continue;
        }
      }

      // This means any value is fine, as long as a key matches.
      // Note that db labels can't have duplicate keys.
      else if (foundAsterikAsValue && newDbLabelKeyMap[key]) {
        matchedLabelMap[key] = true;
        continue;
      }

      // Match against actual values of key and its value.
      else {
        const dbVal = newDbLabelKeyMap[key];
        if (dbVal && vals.find(val => val === dbVal)) {
          matchedLabelMap[key] = true;
          continue;
        }
      }

      // At this point, the current label did not match.
      // This service will not be able to pick up the new db
      // despite any matches so far.
      noMatch = true;
      break matchLabels;
    }

    // See if the current service has all required matching labels.
    if (
      !noMatch &&
      newDbHasLabels &&
      Object.keys(matchedLabelMap).every(key => matchedLabelMap[key])
    ) {
      return currService;
    }
  }

  return null;
}
