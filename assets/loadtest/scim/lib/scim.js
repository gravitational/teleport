/**
 * DEFAULT_RETRY_AFTER_S retry after delay timed in seconds.
 */
const DEFAULT_RETRY_AFTER_S = 1;

const Schema = Object.freeze({
  USER: 'urn:ietf:params:scim:schemas:core:2.0:User',
  GROUP: 'urn:ietf:params:scim:schemas:core:2.0:Group',
  PATCHOP: 'urn:ietf:params:scim:api:messages:2.0:PatchOp',
});

const Path = Object.freeze({
  MEMBERS: 'members',
  USERS: 'Users',
  GROUPS: 'Groups',
});

export const MemberOp = Object.freeze({
  ADD: 'add',
  REMOVE: 'remove',
});

export const HttpMethod = Object.freeze({
  GET: 'GET',
  POST: 'POST',
  PATCH: 'PATCH',
  DELETE: 'DELETE',
});

export const HttpStatus = Object.freeze({
  Ok: 200,
  Created: 201,
  NoContent: 204,
  BadRequest: 400,
  NotFound: 404,
  TooManyRequests: 429,
});

/**
 * SCIM request types used to tag k6 requests.
 */
export const RequestType = Object.freeze({
  TOKEN: 'token',
  CREATE_USER: 'create_user',
  GET_USER: 'get_user',
  DELETE_USER: 'delete_user',
  CREATE_GROUP: 'create_group',
  GET_GROUP: 'get_group',
  PATCH_GROUP: 'patch_group',
});

function patchOp(operation) {
  return { schemas: [Schema.PATCHOP], Operations: [operation] };
}

/**
 * memberPatch builds a PatchOp body in the shape SailPoint sends - one operation touching exactly one member.
 */
export function memberPatch(op, member) {
  switch (op) {
    case MemberOp.ADD:
      return patchOp({ op, path: Path.MEMBERS, value: [{ value: member }] });
    case MemberOp.REMOVE:
      return patchOp({ op, path: `${Path.MEMBERS}[value eq "${member}"]` });
    default:
      throw new Error(`unsupported member op: ${op}`);
  }
}

/**
 * Client implements a minimal SCIM 2.0 client to test Teleport SCIM server.
 */
export class Client {
  constructor(
    { baseUrl, timeout, maxRetries = 0 },
    { transport, tokens, sleep, onRetry = () => {} }
  ) {
    this.baseUrl = baseUrl;
    this.timeout = timeout;
    this.maxRetries = maxRetries;
    this.transport = transport;
    this.tokens = tokens;
    this.sleep = sleep;
    this.onRetry = onRetry;
  }
  // createUser creates a Teleport user.
  // Teleport SCIM server updates user if it already exists.
  createUser(userName) {
    const body = { schemas: [Schema.USER], userName, active: true };
    return this.#send(
      RequestType.CREATE_USER,
      HttpMethod.POST,
      `${this.baseUrl}/${Path.USERS}`,
      body
    );
  }

  // getUser reads a Teleport user.
  getUser(userName) {
    return this.#send(
      RequestType.GET_USER,
      HttpMethod.GET,
      this.#userUrl(userName)
    );
  }

  // deleteUser deletes a Teleport user.
  deleteUser(userName) {
    return this.#send(
      RequestType.DELETE_USER,
      HttpMethod.DELETE,
      this.#userUrl(userName)
    );
  }

  // createGroup links a SCIM group to the existing `type: scim` Access List
  // whose title equals displayName. Teleport does not create the list.
  createGroup(displayName) {
    const body = { schemas: [Schema.GROUP], displayName, members: [] };
    return this.#send(
      RequestType.CREATE_GROUP,
      HttpMethod.POST,
      `${this.baseUrl}/${Path.GROUPS}`,
      body
    );
  }

  getGroup(id) {
    return this.#send(
      RequestType.GET_GROUP,
      HttpMethod.GET,
      this.#groupUrl(id)
    );
  }

  patchGroup(id, body) {
    return this.#send(
      RequestType.PATCH_GROUP,
      HttpMethod.PATCH,
      this.#groupUrl(id),
      body
    );
  }

  #send(requestType, method, url, body) {
    const payload = body === undefined ? null : JSON.stringify(body);
    const retryAfterHeader = 'Retry-After';
    for (let attempt = 0; ; attempt++) {
      const res = this.transport.request(method, url, payload, {
        headers: this.#headers(),
        timeout: this.timeout,
        requestType,
      });
      if (
        res.status !== HttpStatus.TooManyRequests ||
        attempt >= this.maxRetries
      ) {
        return res;
      }
      this.onRetry(requestType);
      this.sleep(
        parseInt(res.headers[retryAfterHeader], 10) || DEFAULT_RETRY_AFTER_S
      );
    }
  }

  #headers() {
    const contentType = 'application/scim+json';
    return {
      Authorization: `Bearer ${this.tokens.token()}`,
      'Content-Type': contentType,
      Accept: contentType,
    };
  }

  #userUrl(userName) {
    return `${this.baseUrl}/${Path.USERS}/${encodeURIComponent(userName)}`;
  }

  #groupUrl(id) {
    return `${this.baseUrl}/${Path.GROUPS}/${encodeURIComponent(id)}`;
  }
}
