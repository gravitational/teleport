import React from 'react';
import WaitingRoom from './WaitingRoom';
import { ContextProvider } from 'teleport';
import TeleportContextE from 'e-teleport/teleportContextE';
import { makeUserContext } from 'teleport/services/user';
import { render, screen, wait, fireEvent } from 'design/utils/testing';
import historyService from 'teleport/services/history';
import { makeAccessRequest } from 'e-teleport/services/workflow';

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
    await wait(() => expect(screen.getByText(/hello/i)).toBeInTheDocument());
  });

  test('strategy "reason" dialog', async () => {
    const userContext = makeUserContext(sampleContext('reason'));
    userContext.accessStrategy.prompt = 'custom prompt';

    jest.spyOn(userService, 'fetchUserContext').mockResolvedValue(userContext);

    render(<>{Component}</>);
    await wait(() =>
      expect(screen.getByText(/custom prompt/i)).toBeInTheDocument()
    );
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
    await wait(() =>
      expect(screen.getByText(/send request/i)).toBeInTheDocument()
    );

    fireEvent.change(screen.getByPlaceholderText(/describe/i), {
      target: { value: 'reason' },
    });

    await wait(() => fireEvent.click(screen.getByText(/send request/i)));
    expect(screen.getByText(/being authorized/i)).toBeInTheDocument();
    expect(workflowService.createAccessRequest).toHaveBeenCalledTimes(1);
    expect(workflowService.fetchAccessRequest).toHaveBeenCalled();
  });

  test('strategy "reason" submit action error', async () => {
    const userContext = makeUserContext(sampleContext('reason'));
    const err = new Error('some error');

    jest.spyOn(workflowService, 'createAccessRequest').mockRejectedValue(err);
    jest.spyOn(userService, 'fetchUserContext').mockResolvedValue(userContext);

    render(<>{Component}</>);
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
    await wait(() => {
      expect(screen.getByText(/being authorized/i)).toBeInTheDocument();
    });

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
    await wait(() => {
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
    await wait(() =>
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
    await wait(() =>
      expect(screen.getByText(/request denied/i)).toBeInTheDocument()
    );
  });

  test('strategy "always" with request APPLIED', async () => {
    const request = makeAccessRequest({ ...sampleRequest, state: 'APPLIED' });
    const userContext = makeUserContext(sampleContext('always'));

    jest.spyOn(storeAccessRequests, 'getWaitingRoom').mockReturnValue(request);
    jest.spyOn(userService, 'fetchUserContext').mockResolvedValue(userContext);

    render(<>{Component}</>);
    await wait(() => expect(screen.getByText(/hello/i)).toBeInTheDocument());
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
    await wait(() =>
      expect(screen.getByText(/error has occurred/i)).toBeInTheDocument()
    );
  });

  test('when user has assumed roles, waiting room is skipped', async () => {
    const request = makeAccessRequest();
    const userContext = makeUserContext(sampleContext('always'));

    jest
      .spyOn(storeAccessRequests, 'getAssumedRoles')
      .mockReturnValue([request]);
    jest.spyOn(storeAccessRequests, 'getWaitingRoom').mockReturnValue(request);
    jest.spyOn(userService, 'fetchUserContext').mockResolvedValue(userContext);

    render(<>{Component}</>);
    await wait(() => expect(screen.getByText(/hello/i)).toBeInTheDocument());
  });
});

const sampleRequest = makeAccessRequest();

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
