/**
 * Create, get or delete users.
 */

import { check } from 'k6';
import exec from 'k6/execution';
import { Counter } from 'k6/metrics';

import {
  env,
  loadScimClientConfig,
  newClient,
  REQUEST_TAG,
  timings,
} from './client.js';
import { UserConfig, UserOp } from './lib/config.js';
import { userName } from './lib/fixture.js';
import { HttpStatus, RequestType } from './lib/scim.js';

/**
 * MAX_TEST_DURATION caps how long to run the test.
 */
const MAX_TEST_DURATION = '2h';

/**
 * usersSucceeded counts successful creates, gets and deletes.
 */
const usersSucceeded = new Counter('scim_users_succeeded');

/**
 * usersNotFound counts delete attempt responded with 404, which can mean
 * either user does not exist or the user is not SCIM managed.
 * They still count as succeeded to differentiate hard failures.
 */
const usersNotFound = new Counter('scim_users_not_found');

const cfg = loadScimClientConfig();
const scim = newClient(cfg);
const userConfig = UserConfig.fromEnv(env);

/**
 * userOps maps each OP to its request type.
 */
const userOps = {
  [UserOp.CREATE]: {
    requestType: RequestType.CREATE_USER,
    send: user => scim.createUser(user),
    okStatuses: [HttpStatus.Created],
  },
  [UserOp.GET]: {
    requestType: RequestType.GET_USER,
    send: user => scim.getUser(user),
    okStatuses: [HttpStatus.Ok],
  },
  [UserOp.DELETE]: {
    requestType: RequestType.DELETE_USER,
    send: user => scim.deleteUser(user),
    okStatuses: [HttpStatus.NoContent, HttpStatus.NotFound],
  },
};
const userOp = userOps[userConfig.op];

// VU cannot be larger than iteration so use count if that is smaller than configured VUS.
const vus = Math.min(userConfig.vus, userConfig.count);

export const options = {
  scenarios: {
    [`${userConfig.op}_users`]: {
      executor: 'shared-iterations',
      vus,
      iterations: userConfig.count,
      maxDuration: MAX_TEST_DURATION,
    },
  },
  thresholds: {
    // Fail the run if create/get/delete request failed.
    checks: ['rate==1.0'],
    // Fail the run if no create/get/delete succeeded, e.g. every request got a 429.
    scim_users_succeeded: ['count>0'],
    // Fail the run if MAX_TEST_DURATION ended it before every user was processed.
    dropped_iterations: ['count==0'],
    // Deletes of users that were not found (404). They are included in the summary.
    scim_users_not_found: ['count>=0'],
    [`http_reqs{${REQUEST_TAG}:${userOp.requestType},status:429}`]: [
      'count>=0',
    ],
    // 429s retried after Retry-After (MAX_RETRIES > 0).
    scim_retries: ['count>=0'],
  },
};

export function setup() {
  console.log(
    `${userConfig.op}: ${userConfig.count} users ` +
      `${userName(userConfig.start)} to ${userName(userConfig.start + userConfig.count - 1)}`
  );
}

export default function () {
  const user = userName(userConfig.start + exec.scenario.iterationInTest);
  const res = userOp.send(user);

  const ok = userOp.okStatuses.includes(res.status);
  usersSucceeded.add(ok ? 1 : 0);
  // Only a delete counts 404 as success.
  usersNotFound.add(
    userConfig.op === UserOp.DELETE && res.status === HttpStatus.NotFound
      ? 1
      : 0
  );

  // A 429 that is still there after MAX_RETRIES retries fails the check.
  // A 429 that a retry recovers from passes, and shows only in scim_retries.
  check(res, { [`${userConfig.op} user`]: () => ok });

  if (!ok) {
    console.log(
      `${userConfig.op} user ${user} failed: ${res.status} ${res.body}, ${timings(res)}`
    );
  }
}
