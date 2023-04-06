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

import { matchLabels, makeLabelMaps } from '../util';

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
  const [timedOut, setTimedOut] = useState(false);

  // Required persisted states to determine if we can skip a request
  // because there can be multiple failed points:
  //  1) failed to create database (network, duplicate name, incorrect format etc)
  //  2) failed to fetch db services (probably mostly network issues)
  //  3) failed to query if there was a matching database service:
  //    - timed out due to combined previous requests taking longer than WAITING_TIMEOUT
  //    - timed out due to failure to query (this would most likely be some kind of
  //      backend error or network failure)
  const [createdDb, setCreatedDb] = useState<CreateDatabaseRequest>();

  const result = usePoll<DatabaseResource>(
    signal => fetchDatabaseServer(signal),
    pollActive,
    3000 // interval: poll every 3 seconds
  );

  // Handles polling timeout.
  useEffect(() => {
    if (pollActive && pollTimeout > Date.now()) {
      const id = window.setTimeout(() => {
        setTimedOut(true);
      }, pollTimeout - Date.now());

      return () => clearTimeout(id);
    }
  }, [pollActive, pollTimeout]);

  useEffect(() => {
    if (timedOut) {
      // reset timer fields and set errors.
      setPollTimeout(null);
      setPollActive(false);
      setTimedOut(false);
      setAttempt({
        status: 'failed',
        statusText:
          'Teleport could not detect your new database in time. Please try again.',
      });
      emitErrorEvent(
        `timeout polling for new database with an existing service`
      );
    }
  }, [timedOut]);

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

  function handleRequestError(err: Error, preErrMsg = '') {
    let message = 'something went wrong';
    if (err instanceof Error) message = err.message;
    setAttempt({ status: 'failed', statusText: message });
    emitErrorEvent(`${preErrMsg}${message}`);
  }

  const access = ctx.storeUser.getDatabaseAccess();
  const resource = props.resourceSpec;
  return {
    attempt,
    clearAttempt,
    registerDatabase,
    canCreateDatabase: access.create,
    pollTimeout,
    dbEngine: resource.dbMeta.engine,
    dbLocation: resource.dbMeta.location,
    isDbCreateErr,
    prevStep: props.prevStep,
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

  // Create maps for easy lookup and matching.
  const { labelKeysToMatchMap, labelValsToMatchMap, labelToMatchSeenMap } =
    makeLabelMaps(newDbLabels);

  const hasLabelsToMatch = newDbLabels.length > 0;
  for (let i = 0; i < dbServices.length; i++) {
    // Loop through the current service label keys and its value set.
    const currService = dbServices[i];
    const match = matchLabels({
      hasLabelsToMatch,
      labelKeysToMatchMap,
      labelValsToMatchMap,
      labelToMatchSeenMap,
      matcherLabels: currService.matcherLabels,
    });

    if (match) {
      return currService;
    }
  }

  return null;
}
