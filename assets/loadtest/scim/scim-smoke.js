/**
 * scim-smoke.js implements a minimal user and member op to test SCIM configuration.
 */

import { check, fail } from 'k6';

import { loadScimClientConfig, newClient, timings } from './client.js';
import { SMOKE_GROUP_NAME, SMOKE_USER_NAME } from './lib/fixture.js';
import { HttpStatus, MemberOp, memberPatch } from './lib/scim.js';

const scim = newClient(loadScimClientConfig());

export const options = {
  vus: 1,
  iterations: 1,
  thresholds: {
    checks: ['rate==1.0'],
  },
};

export function setup() {
  const group = scim.createGroup(SMOKE_GROUP_NAME);
  if (group.status !== HttpStatus.Created) {
    fail(
      `create an Access List first for ${SMOKE_GROUP_NAME}: ${group.status} ${group.body}, ${timings(group)}\n`
    );
  }
  const groupId = group.json('id');
  console.log(
    `group ${SMOKE_GROUP_NAME} linked to Teleport Access List, id=${groupId}, ${timings(group)}`
  );

  const res = scim.createUser(SMOKE_USER_NAME);
  check(res, {
    'create user 201': r => r.status === HttpStatus.Created,
  });
  if (res.status >= 400) {
    console.log(
      `create user ${SMOKE_USER_NAME} failed: ${res.status} ${res.body}, ${timings(res)}`
    );
  }

  return { groupId };
}

/**
 * teardown removes the member and deletes the user. It does not delete the
 * SCIM Access List in Teleport.
 */
export function teardown({ groupId }) {
  patchMember(groupId, MemberOp.REMOVE);

  const removed = memberSet(groupId);
  check(removed, {
    'member removed': m => m !== null && !m.has(SMOKE_USER_NAME),
  });

  const res = scim.deleteUser(SMOKE_USER_NAME);
  check(res, {
    'delete user 204': r => r.status === HttpStatus.NoContent,
  });
  if (res.status >= 400) {
    console.log(
      `delete user ${SMOKE_USER_NAME} failed: ${res.status} ${res.body}, ${timings(res)}`
    );
  }
}

export default function ({ groupId }) {
  patchMember(groupId, MemberOp.ADD);

  const added = memberSet(groupId);
  check(added, {
    'member added': m => m !== null && m.has(SMOKE_USER_NAME),
  });
}

function patchMember(groupId, op) {
  const res = scim.patchGroup(groupId, memberPatch(op, SMOKE_USER_NAME));
  check(res, {
    [`PATCH ${op} 200/204`]: r =>
      r.status === HttpStatus.Ok || r.status === HttpStatus.NoContent,
  });

  const retryAfter = res.headers['Retry-After'];
  console.log(
    `PATCH ${op} ${SMOKE_USER_NAME}: status=${res.status}, ${timings(res)}` +
      (retryAfter ? ` retry-after=${retryAfter}s` : '')
  );
  if (res.status >= 400) {
    console.log(
      `PATCH ${op} failed: ${res.status} ${res.body}, ${timings(res)}`
    );
  }
}

function memberSet(groupId) {
  const res = scim.getGroup(groupId);
  check(res, { 'GET group 200': r => r.status === HttpStatus.Ok });
  if (res.status >= 400) {
    console.log(
      `GET group ${groupId} failed: ${res.status} ${res.body}, ${timings(res)}`
    );
  }

  if (res.status !== HttpStatus.Ok) {
    return null;
  }
  const members = res.json('members') || [];
  console.log(`group ${groupId}: ${members.length} members, ${timings(res)}`);

  return new Set(members.map(m => m.value));
}
