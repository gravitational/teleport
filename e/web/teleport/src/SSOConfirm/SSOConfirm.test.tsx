import { render, screen } from 'design/utils/testing';

import { MemoryRouter } from 'react-router';

import { SSOConfirm } from './SSOConfirm';

class MockBroadcastChannel {
  static instances: Map<string, MockBroadcastChannel> = new Map();
  constructor(name: string) {
    this.name = name;
    MockBroadcastChannel.instances[name] = this;
  }
  postMessage = jest.fn();
  name: string;
  close = jest.fn();
  addEventListener = jest.fn();
  removeEventListener = jest.fn();
  dispatchEvent = jest.fn();
  onmessage = jest.fn();
  onmessageerror = jest.fn();
}

beforeAll(() => {
  global.BroadcastChannel = MockBroadcastChannel;
});

beforeEach(() => {
  jest.clearAllMocks();
});

afterAll(() => {
  delete global.BroadcastChannel;
});

test('renders error if channel id or mfa_token do not exist', async () => {
  const { rerender } = render(
    <MemoryRouter>
      <SSOConfirm />
    </MemoryRouter>
  );

  expect(screen.getByText(/invalid or missing token/i)).toBeInTheDocument();
  rerender(
    <MemoryRouter
      initialEntries={[
        {
          search: '?channel_id=123',
        },
      ]}
    >
      <SSOConfirm />
    </MemoryRouter>
  );
  expect(screen.getByText(/invalid or missing token/i)).toBeInTheDocument();

  rerender(
    <MemoryRouter
      initialEntries={[
        {
          search: '?response={"mfa_token": "token"}',
        },
      ]}
    >
      <SSOConfirm />
    </MemoryRouter>
  );
  expect(screen.getByText(/invalid or missing token/i)).toBeInTheDocument();
});

test('renders success if channel_id and mfa_token exist in response', async () => {
  const channelId = '123';
  const mfaToken = '555';
  render(
    <MemoryRouter
      initialEntries={[
        {
          search: `?channel_id=${channelId}&response={"mfa_token": "${mfaToken}"}`,
        },
      ]}
    >
      <SSOConfirm />
    </MemoryRouter>
  );
  expect(screen.getByText('Authenticated')).toBeInTheDocument();
  expect(MockBroadcastChannel.instances[channelId].name).toBe(channelId);
  expect(
    MockBroadcastChannel.instances[channelId].postMessage
  ).toHaveBeenCalledTimes(1);
  expect(
    MockBroadcastChannel.instances[channelId].postMessage
  ).toHaveBeenCalledWith({ mfaToken });
});
