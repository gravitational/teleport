import React from 'react';
import Banner from './Banner';
import { render, screen, waitFor, fireEvent } from 'design/utils/testing';
import { ContextProvider } from 'teleport';
import history from 'teleport/services/history';
import session from 'teleport/services/session';
import TeleportContextE from 'e-teleport/teleportContextE';

beforeAll(() => {
  jest.useFakeTimers('modern');
  jest.setSystemTime(new Date('2021-04-08T07:00:00Z'));
});

afterAll(() => {
  jest.useRealTimers();
});

describe('banner time display format testing', () => {
  const ctx = new TeleportContextE();
  const store = ctx.storeAccessRequests;
  let Component;

  beforeEach(() => {
    Component = (
      <ContextProvider ctx={ctx}>
        <Banner children={<>hello</>} />
      </ContextProvider>
    );

    jest.spyOn(store, 'getAssumedRoles').mockReturnValue(['dummy']);
  });

  test('hour only, singular', async () => {
    jest
      .spyOn(store, 'getSessionExpiry')
      .mockReturnValue(new Date('2021-04-08T08:00:30Z'));

    render(<>{Component}</>);

    expect(screen.getByText(/expires in 1 hr/)).toBeInTheDocument();
  });

  test('hours and minutes, plural', async () => {
    jest
      .spyOn(store, 'getSessionExpiry')
      .mockReturnValue(new Date('2021-04-08T09:59:00Z'));

    render(<>{Component}</>);

    expect(
      screen.getByText(/expires in 2 hrs and 59 mins/)
    ).toBeInTheDocument();
  });

  test('minutes only', async () => {
    jest
      .spyOn(store, 'getSessionExpiry')
      .mockReturnValue(new Date('2021-04-08T07:30:00Z'));

    render(<>{Component}</>);

    expect(screen.getByText(/expires in 30 mins/)).toBeInTheDocument();
  });

  test('seconds only', async () => {
    jest
      .spyOn(store, 'getSessionExpiry')
      .mockReturnValue(new Date('2021-04-08T07:00:30Z'));

    render(<>{Component}</>);

    expect(screen.getByText(/expires in 30 secs/)).toBeInTheDocument();
  });
});

describe('banner behavioral testing', () => {
  const ctx = new TeleportContextE();
  const workflowSvc = ctx.workflowService;
  const store = ctx.storeAccessRequests;
  const expires = new Date('2021-04-08T08:46:00Z');
  let Component;

  beforeEach(() => {
    Component = (
      <ContextProvider ctx={ctx}>
        <Banner children={<>hello</>} />
      </ContextProvider>
    );
    jest.resetAllMocks();
    jest.spyOn(console, 'error').mockImplementation();
  });

  test('render of banner when date and roles are set', async () => {
    jest.spyOn(store, 'getSessionExpiry').mockReturnValue(expires);
    jest.spyOn(store, 'getAssumedRoles').mockReturnValue(['role1', 'role2']);
    jest.spyOn(store, 'clearAssumes').mockImplementation(null);
    jest.spyOn(workflowSvc, 'applyPermission').mockResolvedValue(null);
    jest.spyOn(history, 'reload').mockImplementation(null);

    render(<>{Component}</>);

    // Test banner renders with correct roles.
    expect(screen.getByText('role1, role2')).toBeInTheDocument();

    // Test children got rendered.
    expect(screen.getByText(/hello/i)).toBeInTheDocument();

    // Test clicking on switchback button, calls appropriate funcs.
    await waitFor(() => fireEvent.click(screen.getByText(/switch back/i)));
    expect(workflowSvc.applyPermission).toHaveBeenCalledWith({
      switchback: true,
    });
    expect(store.clearAssumes).toHaveBeenCalledTimes(1);
    expect(history.reload).toHaveBeenCalledTimes(1);
  });

  test('no banner is rendered when no expiration is set', () => {
    jest.spyOn(store, 'getSessionExpiry').mockReturnValue(null);
    jest.spyOn(store, 'getAssumedRoles').mockReturnValue(['test']);

    render(<>{Component}</>);

    // Test children got rendered.
    expect(screen.getByText(/hello/i)).toBeInTheDocument();

    // Test no banner is present.
    expect(screen.queryByTestId('banner')).toBeNull();
  });

  test('on error, render error dialogue', async () => {
    jest.spyOn(store, 'getSessionExpiry').mockReturnValue(expires);
    jest.spyOn(store, 'getAssumedRoles').mockReturnValue(['dummy']);
    jest
      .spyOn(workflowSvc, 'applyPermission')
      .mockRejectedValue(new Error('some error'));

    render(<>{Component}</>);

    await waitFor(() => fireEvent.click(screen.getByText(/switch back/i)));
    expect(screen.getByTestId('Modal')).toBeInTheDocument();
    expect(screen.getByText('some error')).toBeInTheDocument();
  });

  test('banner button logout when waiting room is set', () => {
    jest.spyOn(session, 'logout').mockImplementation(null);
    jest.spyOn(store, 'getSessionExpiry').mockReturnValue(expires);
    jest.spyOn(store, 'getAssumedRoles').mockReturnValue(['dummy']);
    jest
      .spyOn(store, 'getWaitingRoom')
      .mockReturnValue({ state: 'APPLIED' } as any);

    render(<>{Component}</>);

    fireEvent.click(screen.getByText(/logout/i));
    expect(session.logout).toHaveBeenCalledTimes(1);
  });
});
