import React from 'react';

import { ContextProvider } from 'teleport';

import { makeUserContext } from 'teleport/services/user';
import { render, screen, waitFor, fireEvent } from 'design/utils/testing';
import historyService from 'teleport/services/history';

import {
  AccessRequest,
  makeAccessRequest,
} from 'shared/services/accessRequests';

import TeleportContextE from 'e-teleport/teleportContextE';

import { Container as WaitingRoom } from './WaitingRoom';

beforeAll(() => {
  jest.useFakeTimers();
  jest.setSystemTime(new Date('2020-11-04T19:07:50.693Z'));
});

afterAll(() => {
  jest.useRealTimers();
});

describe('access strategy behavioral testing', () => {
  const ctx = new TeleportContextE();
  const userService = ctx.userService;
  const workflowService = ctx.workflowService;
  const storeAccessRequests = ctx.storeAccessRequests;
  let Component;

  beforeEach(() => {
    Component = (
      <ContextProvider ctx={ctx}>
        <WaitingRoom checkerInterval={0} children={<>hello</>} />
      </ContextProvider>
    );
    jest.resetAllMocks();
    jest.spyOn(console, 'error').mockImplementation();
  });

  afterEach(() => {
    window.localStorage.clear();
  });

  test('strategy "optional"', async () => {
    const userContext = makeUserContext(sampleContext('optional'));

    jest.spyOn(userService, 'fetchUserContext').mockResolvedValue(userContext);

    render(<>{Component}</>);
    await expect(screen.findByText(/hello/i)).resolves.toBeInTheDocument();
  });

  test('strategy "reason" dialog', async () => {
    const userContext = makeUserContext(sampleContext('reason'));
    userContext.accessStrategy.prompt = 'custom prompt';

    jest.spyOn(userService, 'fetchUserContext').mockResolvedValue(userContext);

    render(<>{Component}</>);
    await expect(
      screen.findByText(/custom prompt/i)
    ).resolves.toBeInTheDocument();
  });

  test('strategy "reason" submit action', async () => {
    const request = makeAccessRequest({ ...sampleRequest, state: 'PENDING' });
    const userContext = makeUserContext(sampleContext('reason'));

    jest.spyOn(userService, 'fetchUserContext').mockResolvedValue(userContext);
    jest
      .spyOn(workflowService, 'createAccessRequest')
      .mockResolvedValue(request);
    jest
      .spyOn(workflowService, 'fetchAccessRequest')
      .mockResolvedValue(request);

    render(<>{Component}</>);

    await screen.findByText(/send request/i);

    fireEvent.change(screen.getByPlaceholderText(/describe/i), {
      target: { value: 'reason' },
    });

    fireEvent.click(screen.getByText(/send request/i));
    await screen.findByText(/send request/i);
    await screen.findByText(/being authorized/i);
    expect(workflowService.createAccessRequest).toHaveBeenCalledTimes(1);
    expect(workflowService.fetchAccessRequest).toHaveBeenCalled();
  });

  test('strategy "reason" submit action error', async () => {
    const userContext = makeUserContext(sampleContext('reason'));
    const err = new Error('some error');

    jest.spyOn(workflowService, 'createAccessRequest').mockRejectedValue(err);
    jest.spyOn(userService, 'fetchUserContext').mockResolvedValue(userContext);

    render(<>{Component}</>);
    await screen.findByText(/send request/i);

    fireEvent.change(screen.getByPlaceholderText(/describe/i), {
      target: { value: 'reason' },
    });

    fireEvent.click(screen.getByText(/send request/i));
    expect(screen.getByText(/send request/i)).toBeInTheDocument();
    await expect(screen.findByText(/some error/i)).resolves.toBeInTheDocument();
  });

  test('strategy "always" renders pending dialog, with request state empty and reason required', async () => {
    const userContext = makeUserContext(sampleContext('always'));
    userContext.accessCapabilities.requireReason = true;

    jest.spyOn(userService, 'fetchUserContext').mockResolvedValue(userContext);

    render(<>{Component}</>);
    await expect(
      screen.findByText(/send request/i)
    ).resolves.toBeInTheDocument();
  });

  test('strategy "always" renders pending dialog, with request state empty and reason not required', async () => {
    const request = makeAccessRequest({ ...sampleRequest, state: 'PENDING' });
    const userContext = makeUserContext(sampleContext('always'));

    jest.spyOn(storeAccessRequests, 'setWaitingRoom').mockImplementation();
    jest
      .spyOn(storeAccessRequests, 'getWaitingRoom')
      .mockReturnValueOnce(makeAccessRequest())
      .mockReturnValue(request);
    jest
      .spyOn(workflowService, 'createAccessRequest')
      .mockResolvedValue(request);
    jest
      .spyOn(workflowService, 'fetchAccessRequest')
      .mockResolvedValue(request);
    jest.spyOn(userService, 'fetchUserContext').mockResolvedValue(userContext);

    render(<>{Component}</>);

    await screen.findByText(/being authorized/i);

    expect(workflowService.createAccessRequest).toHaveBeenCalledTimes(1);
    // When access request state is initially empty,
    // hook should auto create request before fetching request.
    expect(workflowService.fetchAccessRequest).toHaveBeenCalled();
    expect(storeAccessRequests.setWaitingRoom).toHaveBeenCalledWith(request);
  });

  test('strategy "always" renders pending dialog, with request state PENDING', async () => {
    const request = makeAccessRequest({ ...sampleRequest, state: 'PENDING' });
    const userContext = makeUserContext(sampleContext('always'));

    jest.spyOn(storeAccessRequests, 'getWaitingRoom').mockReturnValue(request);
    jest
      .spyOn(workflowService, 'createAccessRequest')
      .mockResolvedValue(request);
    jest
      .spyOn(workflowService, 'fetchAccessRequest')
      .mockResolvedValue(request);
    jest.spyOn(userService, 'fetchUserContext').mockResolvedValue(userContext);

    render(<>{Component}</>);
    await waitFor(() => {
      expect(screen.getByText(/being authorized/i)).toBeInTheDocument();
    });

    expect(workflowService.createAccessRequest).not.toHaveBeenCalled();
    expect(workflowService.fetchAccessRequest).toHaveBeenCalled();
  });

  test('strategy "always" with request APPROVED', async () => {
    const request = makeAccessRequest({ ...sampleRequest, state: 'APPROVED' });
    const userContext = makeUserContext(sampleContext('always'));

    jest
      .spyOn(storeAccessRequests, 'setApprovedWaitingRoom')
      .mockImplementation();
    jest.spyOn(storeAccessRequests, 'setWaitingRoom').mockImplementation();
    jest.spyOn(storeAccessRequests, 'getWaitingRoom').mockReturnValue(request);
    jest
      .spyOn(workflowService, 'fetchAccessRequest')
      .mockResolvedValue(request);
    jest.spyOn(workflowService, 'applyPermission').mockResolvedValue(null);
    jest.spyOn(userService, 'fetchUserContext').mockResolvedValue(userContext);
    jest.spyOn(historyService, 'reload').mockImplementation();

    render(<>{Component}</>);
    await waitFor(() =>
      expect(workflowService.applyPermission).toHaveBeenCalledTimes(1)
    );

    // Fetching access request happens with pending,
    // so this proves pending dialog was briefly rendered.
    expect(workflowService.fetchAccessRequest).toHaveBeenCalled();
    expect(historyService.reload).toHaveBeenCalledTimes(1);
    expect(storeAccessRequests.setWaitingRoom).not.toHaveBeenCalled();
    expect(storeAccessRequests.setApprovedWaitingRoom).toHaveBeenCalledTimes(1);
  });

  test('strategy "always" with request DENIED', async () => {
    const request = makeAccessRequest({ ...sampleRequest, state: 'DENIED' });
    const userContext = makeUserContext(sampleContext('always'));

    jest.spyOn(storeAccessRequests, 'getWaitingRoom').mockReturnValue(request);
    jest.spyOn(userService, 'fetchUserContext').mockResolvedValue(userContext);

    render(<>{Component}</>);
    await expect(
      screen.findByText(/request denied/i)
    ).resolves.toBeInTheDocument();
  });

  test('strategy "always" with request APPLIED', async () => {
    const request = makeAccessRequest({ ...sampleRequest, state: 'APPLIED' });
    const userContext = makeUserContext(sampleContext('always'));

    jest.spyOn(storeAccessRequests, 'getWaitingRoom').mockReturnValue(request);
    jest.spyOn(userService, 'fetchUserContext').mockResolvedValue(userContext);

    render(<>{Component}</>);

    await expect(screen.findByText(/hello/i)).resolves.toBeInTheDocument();
  });

  test('strategy "always" fetch request errors', async () => {
    const request = makeAccessRequest({ ...sampleRequest, state: 'APPROVED' });
    const userContext = makeUserContext(sampleContext('always'));

    jest.spyOn(storeAccessRequests, 'getWaitingRoom').mockReturnValue(request);
    jest
      .spyOn(workflowService, 'fetchAccessRequest')
      .mockRejectedValue(new Error('some error'));
    jest.spyOn(userService, 'fetchUserContext').mockResolvedValue(userContext);

    render(<>{Component}</>);
    await expect(
      screen.findByText(/error has occurred/i)
    ).resolves.toBeInTheDocument();
  });

  test('when user has assumed roles, waiting room is skipped', async () => {
    const request = makeAccessRequest({ roles: ['dummy-role'] });
    const userContext = makeUserContext(sampleContext('always'));

    jest
      .spyOn(storeAccessRequests, 'getAssumed')
      .mockReturnValue({ dummyId: request });
    jest.spyOn(storeAccessRequests, 'getWaitingRoom').mockReturnValue(request);
    jest.spyOn(userService, 'fetchUserContext').mockResolvedValue(userContext);

    render(<>{Component}</>);

    await expect(screen.findByText(/hello/i)).resolves.toBeInTheDocument();
  });

  test('date text is formatted corrrectly', async () => {
    const request = makeAccessRequest({
      ...sampleRequest,
      created: '2020-11-04T19:02:50.693Z',
      expires: '2020-11-04T19:17:50.693Z',
    });

    expect(request.createdDuration).toBe('5 minutes ago');
    expect(request.expiresDuration).toBe('10 minutes');
  });
});

const sampleRequest: AccessRequest = {
  id: 'e9803adc-3260-4c49-baae-047494da2822',
  state: 'PENDING',
  resolveReason: '',
  requestReason: '',
  user: 'lisa',
  roles: ['auditor'],
  created: new Date('2024-02-15T02:51:12.700088Z'),
  createdDuration: '',
  expires: new Date('2024-02-17T02:51:12.70087Z'),
  expiresDuration: '',
  maxDuration: new Date('2024-02-17T02:51:12.70087Z'),
  maxDurationText: '',
  requestTTL: new Date('2024-02-15T03:51:12.70087Z'),
  requestTTLDuration: '',
  sessionTTL: new Date('2024-02-15T14:51:03.999893Z'),
  sessionTTLDuration: '',
  reviews: [],
  reviewers: [],
  thresholdNames: ['default'],
  resources: [],
  assumeStartTime: null,
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
