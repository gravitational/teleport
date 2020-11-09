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
  makeUserContext,
  makeAccessRequest,
} from 'teleport/services/user';
import { render, screen, wait, fireEvent } from 'design/utils/testing';
import localStorage from 'teleport/services/localStorage';
import historyService from 'teleport/services/history';

beforeEach(() => {
  jest.resetAllMocks();
  jest.spyOn(console, 'error').mockImplementation();
});

test('strategy "optional"', async () => {
  const userContext = makeUserContext(sampleContext('optional'));

  jest.spyOn(userService, 'fetchUserContext').mockResolvedValue(userContext);

  render(<AccessStrategy children={<>hello</>} />);
  await wait(() => expect(screen.getByText(/hello/i)).toBeInTheDocument());
});

test('strategy "reason" dialog', async () => {
  const userContext = makeUserContext(sampleContext());
  userContext.accessStrategy.type = 'reason';
  userContext.accessStrategy.prompt = 'custom prompt';

  jest.spyOn(userService, 'fetchUserContext').mockResolvedValue(userContext);

  render(<AccessStrategy />);
  await wait(() =>
    expect(screen.getByText(/custom prompt/i)).toBeInTheDocument()
  );
});

test('strategy "reason" submit action', async () => {
  const request = makeAccessRequest({ ...sampleRequest, state: 'PENDING' });
  const userContext = makeUserContext(sampleContext('reason'));

  jest.spyOn(userService, 'fetchUserContext').mockResolvedValue(userContext);
  jest.spyOn(userService, 'createAccessRequest').mockResolvedValue(request);
  jest.spyOn(userService, 'fetchAccessRequest').mockResolvedValue(request);

  render(<AccessStrategy />);
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

test('strategy "reason" submit action error', async () => {
  const userContext = makeUserContext(sampleContext('reason'));
  const err = new Error('some error');
  jest.spyOn(localStorage, 'getAccessRequestResult').mockReturnValue(null);
  jest.spyOn(userService, 'createAccessRequest').mockRejectedValue(err);
  jest.spyOn(userService, 'fetchUserContext').mockResolvedValue(userContext);

  render(<AccessStrategy />);
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

test('strategy "always" renders pending dialog, with request state empty', async () => {
  const request = makeAccessRequest({ ...sampleRequest, state: 'PENDING' });
  const userContext = makeUserContext(sampleContext('always'));

  jest.spyOn(localStorage, 'setAccessRequestResult').mockImplementation();
  jest.spyOn(userService, 'createAccessRequest').mockResolvedValue(request);
  jest.spyOn(userService, 'fetchAccessRequest').mockResolvedValue(request);
  jest.spyOn(userService, 'fetchUserContext').mockResolvedValue(userContext);

  render(<AccessStrategy checkerInterval={0} />);
  await wait(() => {
    expect(screen.getByText(/being authorized/i)).toBeInTheDocument();
  });

  // When access request state is initially empty,
  // hook should auto create request before fetching request.
  expect(userService.createAccessRequest).toHaveBeenCalledTimes(1);
  expect(userService.fetchAccessRequest).toHaveBeenCalled();
  expect(localStorage.setAccessRequestResult).toHaveBeenCalledWith(request);
});

test('strategy "always" renders pending dialog, with request state PENDING', async () => {
  const request = makeAccessRequest({ ...sampleRequest, state: 'PENDING' });
  const userContext = makeUserContext(sampleContext('always'));

  jest
    .spyOn(localStorage, 'getAccessRequestResult')
    .mockReturnValueOnce(request);
  jest.spyOn(userService, 'createAccessRequest').mockResolvedValue(request);
  jest.spyOn(userService, 'fetchAccessRequest').mockResolvedValue(request);
  jest.spyOn(userService, 'fetchUserContext').mockResolvedValue(userContext);

  await wait(() => render(<AccessStrategy checkerInterval={0} />));
  expect(screen.getByText(/being authorized/i)).toBeInTheDocument();

  expect(userService.createAccessRequest).not.toHaveBeenCalled();
  expect(userService.fetchAccessRequest).toHaveBeenCalled();
});

test('strategy "always" with request APPROVED', async () => {
  const request = makeAccessRequest({ ...sampleRequest, state: 'APPROVED' });
  const userContext = makeUserContext(sampleContext('always'));

  jest.spyOn(localStorage, 'setAccessRequestResult').mockImplementation();
  jest.spyOn(localStorage, 'getAccessRequestResult').mockReturnValue(request);
  jest.spyOn(userService, 'fetchAccessRequest').mockResolvedValue(request);
  jest.spyOn(userService, 'fetchUserContext').mockResolvedValue(userContext);
  jest.spyOn(userService, 'applyPermission').mockResolvedValue({});
  jest.spyOn(historyService, 'reload').mockImplementation();

  render(<AccessStrategy checkerInterval={0} children={<>hello</>} />);

  await wait(() =>
    expect(userService.applyPermission).toHaveBeenCalledTimes(1)
  );

  // Fetching access request happens with pending,
  // so this proves pending dialog was briefly rendered.
  expect(userService.fetchAccessRequest).toHaveBeenCalled();
  expect(historyService.reload).toHaveBeenCalledTimes(1);
  expect(localStorage.setAccessRequestResult).toHaveBeenCalledWith({
    ...sampleRequest,
    state: 'APPLIED',
  });
});

test('strategy "always" with request DENIED', async () => {
  const request = makeAccessRequest({ ...sampleRequest, state: 'DENIED' });
  const userContext = makeUserContext(sampleContext('always'));

  jest.spyOn(localStorage, 'getAccessRequestResult').mockReturnValue(request);
  jest.spyOn(userService, 'fetchUserContext').mockResolvedValue(userContext);
  render(<AccessStrategy />);

  await wait(() =>
    expect(screen.getByText(/request denied/i)).toBeInTheDocument()
  );
});

test('strategy "always" with request APPLIED', async () => {
  const request = makeAccessRequest({ ...sampleRequest, state: 'APPLIED' });
  const userContext = makeUserContext(sampleContext('always'));

  jest.spyOn(localStorage, 'getAccessRequestResult').mockReturnValue(request);
  jest.spyOn(userService, 'fetchUserContext').mockResolvedValue(userContext);

  render(<AccessStrategy children={<>hello</>} />);
  await wait(() => expect(screen.getByText(/hello/i)).toBeInTheDocument());
});

test('strategy "always" fetch request errors', async () => {
  const request = makeAccessRequest({ ...sampleRequest, state: 'APPROVED' });
  const userContext = makeUserContext(sampleContext('always'));

  jest.spyOn(localStorage, 'getAccessRequestResult').mockReturnValue(request);
  jest
    .spyOn(userService, 'fetchAccessRequest')
    .mockRejectedValue(new Error('some error'));
  jest.spyOn(userService, 'fetchUserContext').mockResolvedValue(userContext);

  render(<AccessStrategy checkerInterval={0} />);

  await wait(() =>
    expect(screen.getByText(/error has occurred/i)).toBeInTheDocument()
  );
});

const sampleRequest = {
  id: '',
  state: '',
  reason: '',
};

const sampleContext = (type = '') => ({
  accessStrategy: {
    type,
    prompt: '',
  },

  cluster: {
    name: 'im-a-cluster-name',
    lastConnected: '2020-11-04T19:07:50.693Z',
    connectedText: '2020-11-04 11:07:50',
    status: 'online',
    url: '/web/cluster/im-a-cluster-name',
    authVersion: '5.0.0-dev',
    nodeCount: 1,
    publicURL: 'localhost:3080',
    proxyVersion: '5.0.0-dev',
  },
});
