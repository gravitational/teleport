/**
 * Create or Remove group members.
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
import { MemberConfig } from './lib/config.js';
import { groupName } from './lib/fixture.js';
import { HttpStatus, RequestType, memberPatch } from './lib/scim.js';

const cfg = loadScimClientConfig();
const scim = newClient(cfg);
const members = MemberConfig.fromEnv(env);
const op = members.op;

/**
 * MAX_TEST_DURATION caps how long to run the test.
 */
const MAX_TEST_DURATION = '6h';

/**
 * patchesSucceeded counts successful PATCHes (200 or 204).
 */
const patchesSucceeded = new Counter('scim_patches_succeeded');

const memberships = members.memberships();

// VU cannot be larger than iteration so use total if that is smaller than configured VUS.
const vus = Math.min(members.vus, memberships.total);

export const options = {
  scenarios: {
    [`${op}_members`]: {
      executor: 'shared-iterations',
      vus,
      iterations: memberships.total,
      maxDuration: MAX_TEST_DURATION,
    },
  },
  thresholds: {
    checks: ['rate==1.0'],
    // Fails the run if no PATCH succeeded, e.g., every request got a 429.
    scim_patches_succeeded: ['count>0'],
    // Fail the run if MAX_TEST_DURATION ended it before every membership was processed.
    dropped_iterations: ['count==0'],
    // The thresholds below always passes and it only exists to show PATCH duration,
    // the PATCH 429 count and the retries as end of the test summary.
    [`http_req_duration{${REQUEST_TAG}:${RequestType.PATCH_GROUP}}`]: [
      'max>=0',
    ],
    [`http_reqs{${REQUEST_TAG}:${RequestType.PATCH_GROUP},status:429}`]: [
      'count>=0',
    ],
    scim_retries: ['count>=0'],
  },
};

export function setup() {
  console.log(
    `${op}: ${memberships.total} memberships across ${groupName(members.start)} .. ` +
      `${groupName(members.start + members.count - 1)}, ` +
      `${members.membersMin}-${members.membersMax} members each, users pool of ${members.userPool}`
  );
}

export default function () {
  const index = exec.scenario.iterationInTest;
  const { group, user } = memberships.at(index);
  const res = scim.patchGroup(group, memberPatch(op, user));

  // Resp status code 200 returned on a full-state path.
  // Resp status code 204 returned on a newer version that support fast patch.
  const ok =
    res.status === HttpStatus.Ok || res.status === HttpStatus.NoContent;
  patchesSucceeded.add(ok ? 1 : 0);

  // A 429 that is still there after MAX_RETRIES retries fails the check.
  // A 429 that a retry recovers from passes, and shows only in scim_retries.
  check(res, { [`PATCH ${op} 200/204`]: () => ok });

  if (!ok) {
    console.log(
      `PATCH ${op} ${user} -> ${group} (operation ${index}): ${res.status} ${res.body}, ${timings(res)}`
    );
  }
}
