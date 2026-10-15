/**
 * Get groups.
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
import { GroupConfig } from './lib/config.js';
import { groupName } from './lib/fixture.js';
import { HttpStatus, RequestType } from './lib/scim.js';

/**
 * MAX_TEST_DURATION caps how long to run the test.
 */
const MAX_TEST_DURATION = '2h';

/**
 * groupsSucceeded counts successful GETs (200).
 */
const groupsSucceeded = new Counter('scim_groups_succeeded');

const cfg = loadScimClientConfig();
const scim = newClient(cfg);
const groupConfig = GroupConfig.fromEnv(env);

// VU cannot be larger than iteration so use count if that is smaller than configured VUS.
const vus = Math.min(groupConfig.vus, groupConfig.count);

export const options = {
  scenarios: {
    get_groups: {
      executor: 'shared-iterations',
      vus,
      iterations: groupConfig.count,
      maxDuration: MAX_TEST_DURATION,
    },
  },
  thresholds: {
    // Fail the run if a GET failed.
    checks: ['rate==1.0'],
    // Fail the run if no GET succeeded, e.g., every request got a 429.
    scim_groups_succeeded: ['count>0'],
    // Fail the run if MAX_TEST_DURATION ended it before every group was read.
    dropped_iterations: ['count==0'],
    // The thresholds below always pass. They only show the GET duration, the
    // GET 429 count and the retries in the end of test summary.
    [`http_req_duration{${REQUEST_TAG}:${RequestType.GET_GROUP}}`]: ['max>=0'],
    [`http_reqs{${REQUEST_TAG}:${RequestType.GET_GROUP},status:429}`]: [
      'count>=0',
    ],
    scim_retries: ['count>=0'],
  },
};

export function setup() {
  console.log(
    `get: ${groupConfig.count} groups ` +
      `${groupName(groupConfig.start)} to ${groupName(groupConfig.start + groupConfig.count - 1)}`
  );
}

export default function () {
  // The SCIM group ID is the Access List name, which `make acl` sets to the group name.
  const group = groupName(groupConfig.start + exec.scenario.iterationInTest);
  const res = scim.getGroup(group);

  const ok = res.status === HttpStatus.Ok;
  groupsSucceeded.add(ok ? 1 : 0);

  // check fails the run (see the checks threshold) when the GET did not return 200.
  check(res, { 'get group': () => ok });

  if (ok) {
    console.log(
      `get group ${group}: ${res.json('members.#')} members, ${res.body.length} bytes, ${timings(res)}`
    );
  } else {
    console.log(
      `get group ${group} failed: ${res.status} ${res.body}, ${timings(res)}`
    );
  }
}
