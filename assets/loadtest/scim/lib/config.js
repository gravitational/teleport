import {
  groupSizesBetween,
  memberships as buildMemberships,
} from './fixture.js';
import { MemberOp } from './scim.js';

/**
 * DEFAULT_TIMEOUT defines how long k6 waits for HTTP response.
 * 5 minutes may seem too long but this was chosen while testing on full-state
 * patch path where a single member patch to a group with 50k members took about
 * 4 minutes.
 */
const DEFAULT_TIMEOUT = '5m';

/**
 * DEFAULT_MAX_RETRIES defines how many times SCIM client will retry
 * after getting a 429 response. Keep 0 to test when does Teleport
 * start sending 429s under pressure, and increase the value to
 * test sustained 429 request and recovery.
 */
const DEFAULT_MAX_RETRIES = 0;

/**
 * DEFAULT_VUS is the default number of k6 virtual users (VUs), overridden by VUS.
 */
const DEFAULT_VUS = 1;

function optionalInt(env, name) {
  const value = env.optional(name);
  return value && Number(value);
}

function requireInt(name, value, min) {
  if (!Number.isInteger(value) || value < min) {
    throw new Error(`${name} (${value}) must be an integer >= ${min}`);
  }
}

/**
 * ScimClientConfig defines core SCIM client related settings.
 */
export class ScimClientConfig {
  constructor({
    baseUrl,
    tokenUrl,
    clientId,
    clientSecret,
    timeout = DEFAULT_TIMEOUT,
    maxRetries = DEFAULT_MAX_RETRIES,
  }) {
    requireInt('MAX_RETRIES', maxRetries, 0);

    // baseUrl is the Base URL printed by `tctl plugins install scim`.
    this.baseUrl = baseUrl.replace(/\/+$/, '');
    // tokenUrl is the OAuth Token URL printed by `tctl plugins install scim`.
    this.tokenUrl = tokenUrl;
    // timeout is the per-request timeout. E.g., '60s'.
    this.timeout = timeout;
    // maxRetries retries 429 responses after Retry-After. use 0 to disable retries.
    this.maxRetries = maxRetries;
    this.clientId = clientId;
    this.clientSecret = clientSecret;
  }

  static fromEnv(env) {
    return new ScimClientConfig({
      baseUrl: env.required('BASE_URL'),
      tokenUrl: env.required('TOKEN_URL'),
      clientId: env.required('CLIENT_ID'),
      clientSecret: env.required('CLIENT_SECRET'),
      timeout: env.optional('TIMEOUT'),
      maxRetries: optionalInt(env, 'MAX_RETRIES'),
    });
  }
}

/**
 * DEFAULT_USER_START is default first user index.
 * User names are deterministic and are configurable with the start value. For example:
 * - DEFAULT_USER_START=0 means k6 will create user as `k6-user-0@example.com`.
 * - DEFAULT_USER_START=10 means k6 will create user as `k6-user-10@example.com`.
 * Default value is overriden by USER_START env var.
 */
const DEFAULT_USER_START = 0;

/**
 * DEFAULT_USER_COUNT defines the total users to create.
 * E.g., DEFAULT_USER_COUNT=10 means k6 will create 10 users in Teleport.
 * Default value is overriden by USER_COUNT env var.
 */
const DEFAULT_USER_COUNT = 1;

/**
 * UserOp is what scim-users.js does with the selected users, set by OP.
 */
export const UserOp = Object.freeze({
  CREATE: 'create',
  GET: 'get',
  DELETE: 'delete',
});

/**
 * UserConfig defines user related settings.
 */
export class UserConfig {
  constructor({
    start = DEFAULT_USER_START,
    count = DEFAULT_USER_COUNT,
    op = UserOp.CREATE,
    vus = DEFAULT_VUS,
  }) {
    requireInt('USER_START', start, 0);
    requireInt('USER_COUNT', count, 1);
    requireInt('VUS', vus, 1);
    if (!Object.values(UserOp).includes(op)) {
      throw new Error(
        `OP must be one of ${Object.values(UserOp).join(', ')}, got ${op}`
      );
    }
    this.start = start;
    this.count = count;
    // op is whether the users are created, read or deleted.
    this.op = op;
    // vus is the number of concurrent k6 virtual users.
    this.vus = vus;
  }

  static fromEnv(env) {
    return new UserConfig({
      start: optionalInt(env, 'USER_START'),
      count: optionalInt(env, 'USER_COUNT'),
      op: env.optional('OP')?.toLowerCase(),
      vus: optionalInt(env, 'VUS'),
    });
  }
}

/**
 * DEFAULT_GROUP_START is the default first group index, overridden by GROUP_START.
 */
const DEFAULT_GROUP_START = 0;

/**
 * DEFAULT_GROUP_COUNT is the default number of groups, overridden by GROUP_COUNT.
 */
const DEFAULT_GROUP_COUNT = 1;

/**
 * GroupConfig defines the groups that scim-groups.js reads.
 */
export class GroupConfig {
  constructor({
    start = DEFAULT_GROUP_START,
    count = DEFAULT_GROUP_COUNT,
    vus = DEFAULT_VUS,
  }) {
    requireInt('GROUP_START', start, 0);
    requireInt('GROUP_COUNT', count, 1);
    requireInt('VUS', vus, 1);
    // start is the index of the first group, e.g., 99 for k6-group-99.
    this.start = start;
    // count is the number of groups to read.
    this.count = count;
    // vus is the number of concurrent k6 virtual users.
    this.vus = vus;
  }

  static fromEnv(env) {
    return new GroupConfig({
      start: optionalInt(env, 'GROUP_START'),
      count: optionalInt(env, 'GROUP_COUNT'),
      vus: optionalInt(env, 'VUS'),
    });
  }
}

/**
 * DEFAULT_MEMBERS_MIN defines how many members should be added at minimum.
 */
const DEFAULT_MEMBERS_MIN = 5;

/**
 * DEFAULT_MEMBERS_MAX defines how many members should be added at maximum.
 */
const DEFAULT_MEMBERS_MAX = 5;

/**
 * MemberConfig defines member related settings.
 */
export class MemberConfig {
  constructor({
    start = DEFAULT_GROUP_START,
    count = DEFAULT_GROUP_COUNT,
    membersMin = DEFAULT_MEMBERS_MIN,
    membersMax = DEFAULT_MEMBERS_MAX,
    userPool = membersMax,
    op = MemberOp.ADD,
    vus = DEFAULT_VUS,
  }) {
    requireInt('GROUP_START', start, 0);
    requireInt('GROUP_COUNT', count, 1);
    requireInt('MEMBERS_MIN', membersMin, 1);
    requireInt('MEMBERS_MAX', membersMax, 1);
    requireInt('USER_POOL', userPool, 1);
    requireInt('VUS', vus, 1);
    if (!Object.values(MemberOp).includes(op)) {
      throw new Error(
        `OP must be one of ${Object.values(MemberOp).join(', ')}, got ${op}`
      );
    }
    if (membersMin > membersMax) {
      throw new Error(
        `MEMBERS_MIN (${membersMin}) must be <= MEMBERS_MAX (${membersMax})`
      );
    }
    if (userPool < membersMax) {
      throw new Error(
        `USER_POOL (${userPool}) must be >= MEMBERS_MAX (${membersMax})`
      );
    }

    // start defines index of the group.
    // Group names are deterministic.
    // In a group pool of 10 groups, if you want to add members to group at the 6th group,
    // configure GROUP_START=5.
    this.start = start;
    // count is total number of groups where members should be added.
    this.count = count;
    // membersMin number of members that should be added.
    this.membersMin = membersMin;
    // membersMax number of members to be added.
    this.membersMax = membersMax;
    // userPool defines pool of user that can be drawn for member patching.
    // E.g. GROUP_COUNT=5, MEMBERS_MIN=20, MEMBERS_MAX=20, USER_POOL=100: 5 groups of exactly 20 members, drawn from users 0-99.
    this.userPool = userPool;
    // op is whether the memberships are added or removed.
    this.op = op;
    // vus is the number of concurrent k6 virtual users.
    this.vus = vus;
  }

  static fromEnv(env) {
    return new MemberConfig({
      start: optionalInt(env, 'GROUP_START'),
      count: optionalInt(env, 'GROUP_COUNT'),
      membersMin: optionalInt(env, 'MEMBERS_MIN'),
      membersMax: optionalInt(env, 'MEMBERS_MAX'),
      userPool: optionalInt(env, 'USER_POOL'),
      op: env.optional('OP')?.toLowerCase(),
      vus: optionalInt(env, 'VUS'),
    });
  }

  // memberships returns the selected memberships, in order (see memberships in fixture.js).
  memberships() {
    const sizes = groupSizesBetween(
      this.start,
      this.count,
      this.membersMin,
      this.membersMax
    );
    return buildMemberships(this.start, sizes, this.userPool);
  }
}
