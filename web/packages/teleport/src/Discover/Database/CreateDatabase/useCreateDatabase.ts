/**
 * Teleport
 * Copyright (C) 2023  Gravitational, Inc.
 *
 * This program is free software: you can redistribute it and/or modify
 * it under the terms of the GNU Affero General Public License as published by
 * the Free Software Foundation, either version 3 of the License, or
 * (at your option) any later version.
 *
 * This program is distributed in the hope that it will be useful,
 * but WITHOUT ANY WARRANTY; without even the implied warranty of
 * MERCHANTABILITY or FITNESS FOR A PARTICULAR PURPOSE.  See the
 * GNU Affero General Public License for more details.
 *
 * You should have received a copy of the GNU Affero General Public License
 * along with this program.  If not, see <http://www.gnu.org/licenses/>.
 */

import { useEffect, useState } from 'react';

import useAttempt from 'shared/hooks/useAttemptNext';
import { getErrMessage } from 'shared/utils/errorType';

import cfg from 'teleport/config';
import { DatabaseLocation } from 'teleport/Discover/SelectResource';
import { usePoll } from 'teleport/Discover/Shared/usePoll';
import { useDiscover, type DbMeta } from 'teleport/Discover/useDiscover';
import { compareByString } from 'teleport/lib/util';
import type { ResourceLabel } from 'teleport/services/agents';
import { ApiError } from 'teleport/services/api/parseError';
import {
  IamPolicyStatus,
  type CreateDatabaseRequest,
  type Database as DatabaseResource,
  type DatabaseService,
} from 'teleport/services/databases';
import useTeleport from 'teleport/useTeleport';

import { matchLabels } from '../common';
import { dbWithoutDbServerExistsErrorMsg, timeoutErrorMsg } from './const';

export const WAITING_TIMEOUT = 30000; // 30 seconds

export function useCreateDatabase() {
  const ctx = useTeleport();
  const clusterId = ctx.storeUser.getClusterId();
  const { attempt, setAttempt } = useAttempt('');
  const {
    emitErrorEvent,
    updateAgentMeta,
    agentMeta,
    nextStep,
    prevStep,
    resourceSpec,
  } = useDiscover();

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

  const isAws = resourceSpec.dbMeta.location === DatabaseLocation.Aws;

  const dbPollingResult = usePoll<DatabaseResource>(
    signal => fetchDatabaseServer(signal),
    pollActive, // does not poll on init, since the value is false.
    3000 // interval: poll every 3 seconds
  );

  // Handles setting a timeout when polling becomes active.
  useEffect(() => {
    if (pollActive && pollTimeout > Date.now()) {
      const id = window.setTimeout(() => {
        setTimedOut(true);
      }, pollTimeout - Date.now());

      return () => clearTimeout(id);
    }
  }, [pollActive, pollTimeout]);

  // Handles polling timeout.
  useEffect(() => {
    if (timedOut) {
      // reset timer fields and set errors.
      setPollTimeout(null);
      setPollActive(false);
      setTimedOut(false);
      setAttempt({
        status: 'failed',
        statusText: timeoutErrorMsg,
      });
      emitErrorEvent(
        `timeout polling for new database with an existing service`
      );
    }
  }, [timedOut]);

  // Handles when polling successfully gets
  // a response.
  useEffect(() => {
    if (!dbPollingResult) return;

    setPollTimeout(null);
    setPollActive(false);

    updateAgentMeta({
      ...(agentMeta as DbMeta),
      resourceName: createdDb.name,
      awsRegion: createdDb.awsRegion,
      awsVpcId: createdDb.awsVpcId,
      agentMatcherLabels: dbPollingResult.labels,
      db: dbPollingResult,
      serviceDeployedMethod:
        dbPollingResult.aws?.iamPolicyStatus === IamPolicyStatus.Success
          ? 'skipped'
          : undefined, // User has to deploy a service (can be auto or manual)
    });

    setAttempt({ status: 'success' });
  }, [dbPollingResult]);

  function handleOnTimeout() {
    updateAgentMetaUponRequiringDeployment(createdDb);
  }

  function updateAgentMetaUponRequiringDeployment(db: CreateDatabaseRequest) {
    updateAgentMeta({
      ...(agentMeta as DbMeta),
      resourceName: db.name,
      awsRegion: db.awsRegion,
      agentMatcherLabels: db.labels,
      selectedAwsRdsDb: db.awsRds,
      awsVpcId: db.awsVpcId,
    });
    handleNextStep();
  }

  // fetchDatabaseServer is the callback that is run every interval by the poller.
  // The poller will stop polling once a result returns (a dbServer).
  function fetchDatabaseServer(signal: AbortSignal) {
    const request = {
      search: createdDb.name,
      limit: 1,
    };
    return ctx.databaseService
      .fetchDatabases(clusterId, request, signal)
      .then(res => {
        if (res.agents.length) {
          const dbServer = res.agents[0];
          if (
            !isAws || // If not AWS, then we return the first thing we get back.
            // If AWS and aws.iamPolicyStatus is undefined or non-pending,
            // return the dbServer.
            dbServer.aws?.iamPolicyStatus !== IamPolicyStatus.Pending
          ) {
            return dbServer;
          }
        }
        // Returning nothing here will continue the polling.
        // Either no result came back back yet or
        // a result did come back but we are waiting for a specific
        // marker to appear in the result. Specifically for AWS dbs,
        // we wait for a non-pending flag to appear.
        return null;
      });
  }

  function fetchDatabaseServers(query: string) {
    const request = {
      query,
    };
    return ctx.databaseService.fetchDatabases(clusterId, request);
  }

  async function registerDatabase(
    db: CreateDatabaseRequest,
    { newDb = false, overwriteDb = false } = {}
  ) {
    // Set the timeout now, because this entire registering process
    // should take less than WAITING_TIMEOUT.
    setPollTimeout(Date.now() + WAITING_TIMEOUT);
    setAttempt({ status: 'processing' });
    setIsDbCreateErr(false);

    if (overwriteDb) {
      try {
        await ctx.databaseService.createDatabase(clusterId, {
          ...db,
          overwrite: true,
        });
        setCreatedDb(db);
      } catch (err) {
        handleRequestError(err, 'failed to overwrite database: ');
        return;
      }
    }
    // Attempt creating a new Database resource.
    else if (!createdDb || newDb) {
      try {
        await ctx.databaseService.createDatabase(clusterId, db);
        setCreatedDb(db);
      } catch (err) {
        // Check if the error is a result of an existing database.
        if (err instanceof ApiError) {
          if (err.response.status === 409) {
            const isAwsRds = Boolean(db.awsRds && db.awsRds.accountId);
            return attemptDbServerQueryAndBuildErrMsg(db.name, isAwsRds);
          }
        }
        handleRequestError(err, 'failed to create database: ');
        setIsDbCreateErr(true);
        return;
      }
    }
    // Check and see if database resource need to be updated.
    else if (requiresDbUpdate(db)) {
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
      const { services } =
        await ctx.databaseService.fetchDatabaseServices(clusterId);

      if (!findActiveDatabaseSvc(db.labels, services)) {
        updateAgentMetaUponRequiringDeployment(db);
        setAttempt({ status: 'success' });
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

  // attemptDbServerQueryAndBuildErrMsg tests if the duplicated `dbName`
  // (determined by an error returned from the initial register db attempt)
  // is already a part of the cluster by querying for its db server.
  // This is an attempt to provide accurate actionable steps for the
  // user.
  async function attemptDbServerQueryAndBuildErrMsg(
    dbName: string,
    isAwsRds = false
  ) {
    const selfHostedMsg = `use a different name and retry.`;
    const awsMsg = `alternatively upsert the value of the \
    AWS tag "TeleportDatabaseName" on the RDS instance and retry.`;

    try {
      await ctx.databaseService.fetchDatabase(clusterId, dbName);
      let message = `A database with the name "${dbName}" is already \
      a part of this cluster, ${isAwsRds ? awsMsg : selfHostedMsg}`;
      handleRequestError(new Error(message));
    } catch (e) {
      // No database server were found for the database name
      // so it'll be safe to overwrite the database.
      if (e instanceof ApiError) {
        if (e.response.status === 404) {
          let message = `A database with the name "${dbName}" ${dbWithoutDbServerExistsErrorMsg}. \
          You can overwrite it, or ${isAwsRds ? awsMsg : selfHostedMsg}`;
          handleRequestError(new Error(message));
        }
        return;
      }

      // Display other errors as is.
      handleRequestError(e, 'failed to register database:');
    }
    setIsDbCreateErr(true);
  }

  function requiresDbUpdate(db: CreateDatabaseRequest) {
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
      createdDb.awsRds?.resourceId !== db.awsRds?.resourceId ||
      createdDb.awsRds?.vpcId !== db.awsRds?.vpcId ||
      createdDb.awsRds?.securityGroups !== db.awsRds?.securityGroups ||
      createdDb.awsRds?.subnets !== db.awsRds?.subnets ||
      createdDb.awsRds?.accountId !== db.awsRds?.accountId
    );
  }

  function clearAttempt() {
    setAttempt({ status: '' });
  }

  function handleRequestError(err: Error, preErrMsg = '') {
    const message = getErrMessage(err);
    setAttempt({ status: 'failed', statusText: `${preErrMsg}${message}` });
    emitErrorEvent(`${preErrMsg}${message}`);
  }

  function handleNextStep() {
    if (isAws && !cfg.isCloud) {
      handleNextStepForSelfHostedAwsEnrollment();
      return;
    }

    if (dbPollingResult) {
      if (
        isAws &&
        dbPollingResult.aws?.iamPolicyStatus === IamPolicyStatus.Success
      ) {
        // Skips the deploy db service step AND setting up IAM policy step.
        return nextStep(3);
      }
      // Skips the deploy database service step.
      return nextStep(2);
    }
    nextStep(); // Goes to deploy database service step.
  }

  /**
   * self hosted AWS enrollment flow has one additional step
   * called the Configure Discovery Service. This step is
   * only required if user enabled auto discovery.
   * If a user is here in "useCreateDatabase" then user did not
   * opt for auto discovery (auto discovery will auto create dbs),
   * so we need to skip this step here.
   */
  function handleNextStepForSelfHostedAwsEnrollment() {
    if (dbPollingResult) {
      if (dbPollingResult.aws?.iamPolicyStatus === IamPolicyStatus.Success) {
        // Skips configure discovery service, deploy db service AND
        // setting up IAM policy step
        return nextStep(4);
      }
      // Skips the configure discovery service and deploy database service step.
      return nextStep(3);
    }
    nextStep(2); // Skips the discovery service (goes to deploy database service step).
  }

  const access = ctx.storeUser.getDatabaseAccess();
  return {
    createdDb,
    attempt,
    clearAttempt,
    registerDatabase,
    fetchDatabaseServers,
    handleOnTimeout,
    canCreateDatabase: access.create,
    pollTimeout,
    dbEngine: resourceSpec.dbMeta.engine,
    dbLocation: resourceSpec.dbMeta.location,
    isDbCreateErr,
    prevStep,
    nextStep: handleNextStep,
  };
}

export type State = ReturnType<typeof useCreateDatabase>;

export function findActiveDatabaseSvc(
  newDbLabels: ResourceLabel[],
  dbServices: DatabaseService[]
) {
  if (!dbServices.length) {
    return null;
  }

  for (let i = 0; i < dbServices.length; i++) {
    // Loop through the current service label keys and its value set.
    const currService = dbServices[i];
    const match = matchLabels(newDbLabels, currService.matcherLabels);

    if (match) {
      return currService;
    }
  }

  return null;
}
