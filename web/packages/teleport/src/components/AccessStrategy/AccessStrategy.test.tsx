/**
 * Copyright 2020 Gravitational, Inc.
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

import React from 'react';
import AccessStrategy from './AccessStrategy';
import userService, {
  makeUser,
  makeAccessRequest,
} from 'teleport/services/user';
import { render, screen, wait, fireEvent } from 'design/utils/testing';
import sessionStorage from 'teleport/services/localStorage';

beforeEach(() => {
  jest.resetAllMocks();
  sessionStorage.clear();

  jest.spyOn(console, 'error').mockImplementation();
});

test('stategy "optional" renders children', async () => {
  const userContext = makeUser(sample.userContext);

  jest.spyOn(userService, 'fetchUser').mockResolvedValue(userContext);

  render(<AccessStrategy children={sample.children} />);
  await wait(() =>
    expect(screen.getByText(/hello world/i)).toBeInTheDocument()
  );
});

test('strategy "reason" renders reason dialogue with custom prompt', async () => {
  const userContext = makeUser(sample.userContext);
  userContext.accessStrategy.type = 'reason';
  userContext.accessStrategy.prompt = 'custom prompt';

  jest.spyOn(userService, 'fetchUser').mockResolvedValue(userContext);

  render(<AccessStrategy children={sample.children} />);
  await wait(() =>
    expect(screen.getByText(/custom prompt/i)).toBeInTheDocument()
  );
});

test('strategy "reason" renders pending dialogue, on click request', async () => {
  const userContext = makeUser(sample.userContext);
  userContext.accessStrategy.type = 'reason';

  const request = makeAccessRequest({
    id: '',
    state: 'PENDING',
    reason: '',
  });
  jest.spyOn(userService, 'fetchUser').mockResolvedValue(userContext);
  jest.spyOn(userService, 'createAccessRequest').mockResolvedValue(request);
  jest.spyOn(userService, 'fetchAccessRequest').mockResolvedValue(request);

  render(<AccessStrategy children={sample.children} />);
  await wait(() =>
    expect(screen.getByText(/send request/i)).toBeInTheDocument()
  );

  fireEvent.change(screen.getByPlaceholderText(/describe/i), {
    target: { value: 'reason' },
  });

  await wait(() => fireEvent.click(screen.getByText(/send request/i)));
  expect(screen.getByText(/being authorized/i)).toBeInTheDocument();
  expect(userService.createAccessRequest).toHaveBeenCalledTimes(1);
  expect(userService.fetchAccessRequest).toHaveBeenCalled();
});

test('strategy "reason", on create request error, renders alert error banner', async () => {
  const userContext = makeUser(sample.userContext);
  userContext.accessStrategy.type = 'reason';

  const err = new Error('some error');
  jest.spyOn(userService, 'createAccessRequest').mockRejectedValue(err);
  jest.spyOn(userService, 'fetchUser').mockResolvedValue(userContext);

  render(<AccessStrategy children={sample.children} />);
  await wait(() =>
    expect(screen.getByText(/send request/i)).toBeInTheDocument()
  );

  fireEvent.change(screen.getByPlaceholderText(/describe/i), {
    target: { value: 'reason' },
  });

  await wait(() => fireEvent.click(screen.getByText(/send request/i)));
  expect(screen.getByText(/send request/i)).toBeInTheDocument();
  expect(screen.getByText(/some error/i)).toBeInTheDocument();
});

test('strategy "always" renders pending dialogue, with request state empty', async () => {
  const userContext = makeUser(sample.userContext);
  userContext.accessStrategy.type = 'always';

  const request = makeAccessRequest({
    id: '',
    state: 'PENDING',
    reason: '',
  });
  jest.spyOn(userService, 'createAccessRequest').mockResolvedValue(request);
  jest.spyOn(userService, 'fetchAccessRequest').mockResolvedValue(request);
  jest.spyOn(userService, 'fetchUser').mockResolvedValue(userContext);

  expect(sessionStorage.getAccessRequestResult()).toBeNull();

  render(<AccessStrategy children={sample.children} checkerInterval={0} />);
  await wait(() => {
    expect(screen.getByText(/being authorized/i)).toBeInTheDocument();
  });

  // When access request state is initially empty,
  // hook should auto create request before fetching request.
  expect(userService.createAccessRequest).toHaveBeenCalledTimes(1);
  expect(userService.fetchAccessRequest).toHaveBeenCalled();
  expect(sessionStorage.getAccessRequestResult()).toStrictEqual(request);
});

test('strategy "always" renders pending dialogue, with request state PENDING', async () => {
  const userContext = makeUser(sample.userContext);
  userContext.accessStrategy.type = 'always';

  const request = makeAccessRequest({
    id: '',
    state: 'PENDING',
    reason: '',
  });
  sessionStorage.setAccessRequestResult(request);
  expect(sessionStorage.getAccessRequestResult()).toStrictEqual(request);

  jest.spyOn(userService, 'createAccessRequest').mockResolvedValue(request);
  jest.spyOn(userService, 'fetchAccessRequest').mockResolvedValue(request);
  jest.spyOn(userService, 'fetchUser').mockResolvedValue(userContext);

  await wait(() =>
    render(<AccessStrategy children={sample.children} checkerInterval={0} />)
  );
  expect(screen.getByText(/being authorized/i)).toBeInTheDocument();

  expect(userService.createAccessRequest).not.toHaveBeenCalledTimes(1);
  expect(userService.fetchAccessRequest).toHaveBeenCalled();
});

test('strategy "always", renders pending then children, with request state APPROVED', async () => {
  const userContext = makeUser(sample.userContext);
  userContext.accessStrategy.type = 'always';

  const request = makeAccessRequest({
    id: '',
    state: 'APPROVED',
    reason: '',
  });
  sessionStorage.setAccessRequestResult(request);
  expect(sessionStorage.getAccessRequestResult()).toStrictEqual(request);

  jest.spyOn(userService, 'fetchAccessRequest').mockResolvedValue(request);
  jest.spyOn(userService, 'applyPermission').mockResolvedValue({});
  jest.spyOn(userService, 'fetchUser').mockResolvedValue(userContext);
  Object.defineProperty(window, 'location', {
    writable: true,
    value: { reload: jest.fn() },
  });

  render(<AccessStrategy children={sample.children} checkerInterval={0} />);
  await wait(() =>
    expect(screen.getByText(/hello world/i)).toBeInTheDocument()
  );
  expect(userService.applyPermission).toHaveBeenCalledTimes(1);
  expect(window.location.reload).toHaveBeenCalledTimes(1);
  expect(sessionStorage.getAccessRequestResult().state).toEqual('APPLIED');

  // Fetching access request happens with pending,
  // so this prooves pending dialogue was briefly rendered.
  expect(userService.fetchAccessRequest).toHaveBeenCalled();
});

test('strategy "always", renders denied dialogue, with request state DENIED', async () => {
  const userContext = makeUser(sample.userContext);
  userContext.accessStrategy.type = 'always';

  let request = makeAccessRequest({
    id: '',
    state: 'DENIED',
    reason: '',
  });
  sessionStorage.setAccessRequestResult(request);
  expect(sessionStorage.getAccessRequestResult()).toStrictEqual(request);

  jest.spyOn(userService, 'fetchUser').mockResolvedValue(userContext);

  render(<AccessStrategy children={sample.children} />);

  await wait(() =>
    expect(screen.getByText(/request denied/i)).toBeInTheDocument()
  );
});

test('strategy "always", renders children, with request state APPLIED', async () => {
  const userContext = makeUser(sample.userContext);
  userContext.accessStrategy.type = 'always';

  let request = makeAccessRequest({
    id: '',
    state: 'APPLIED',
    reason: '',
  });
  sessionStorage.setAccessRequestResult(request);
  expect(sessionStorage.getAccessRequestResult()).toStrictEqual(request);

  jest.spyOn(userService, 'fetchUser').mockResolvedValue(userContext);

  render(<AccessStrategy children={sample.children} />);

  await wait(() =>
    expect(screen.getByText(/hello world/i)).toBeInTheDocument()
  );
});

test('strategy "always", on fetch request error, renders error dialogue', async () => {
  const userContext = makeUser(sample.userContext);
  userContext.accessStrategy.type = 'always';

  let request = makeAccessRequest({
    id: '',
    state: 'APPROVED',
    reason: '',
  });
  sessionStorage.setAccessRequestResult(request);
  expect(sessionStorage.getAccessRequestResult()).toStrictEqual(request);

  const err = new Error('some error');
  jest.spyOn(userService, 'fetchAccessRequest').mockRejectedValue(err);
  jest.spyOn(userService, 'fetchUser').mockResolvedValue(userContext);

  render(<AccessStrategy children={sample.children} checkerInterval={0} />);

  await wait(() =>
    expect(screen.getByText(/error has occurred/i)).toBeInTheDocument()
  );
});

const sample = {
  checkerInterval: 0,
  userContext: {
    accessStrategy: {
      type: 'optional',
      prompt: '',
    },
    username: 'alice',
    authType: 'local',
    acl: {
      logins: ['root'],
      authConnectors: {
        list: true,
        read: true,
        edit: true,
        create: true,
        remove: true,
      },
      trustedClusters: {
        list: true,
        read: true,
        edit: true,
        create: true,
        remove: true,
      },
      roles: {
        list: true,
        read: true,
        edit: true,
        create: true,
        remove: true,
      },
      sessions: {
        list: true,
        read: true,
        edit: false,
        create: false,
        remove: false,
      },
      events: {
        list: true,
        read: true,
        edit: false,
        create: false,
        remove: false,
      },
    },
    cluster: {
      clusterId: 'im-a-cluster-name',
      lastConnected: '2020-11-04T19:07:50.693Z',
      connectedText: '2020-11-04 11:07:50',
      status: 'online',
      url: '/web/cluster/im-a-cluster-name',
      authVersion: '5.0.0-dev',
      nodeCount: 1,
      publicURL: 'localhost:3080',
      proxyVersion: '5.0.0-dev',
    },
  },
  children: <div>hello world</div>,
};
