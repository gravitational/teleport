import React from 'react';
import renderHook, { act } from 'design/utils/renderHook';
import { render } from 'design/utils/testing';
import { Context } from 'teleport';
import makeAcl from 'teleport/services/user/makeAcl';
import { apps } from './fixtures';
import useApps from './useApps';
import { Loaded, Failed, Empty, EmptyReadOnly } from './Apps.story';

jest.mock('teleport/useStickyClusterId', () =>
  jest.fn(() => ({ clusterId: 'im-a-cluster', isLeafCluster: false }))
);

test('loaded state', async () => {
  const { container, findAllByText } = render(<Loaded />);
  await findAllByText(/Applications/i);

  expect(container).toMatchSnapshot();
});

test('failed state', async () => {
  const { container, findAllByText } = render(<Failed />);
  await findAllByText(/some error message/i);

  expect(container).toMatchSnapshot();
});

test('empty state for enterprise, can create', () => {
  const { container } = render(<Empty />);
  expect(container).toMatchSnapshot();
});

test('readonly empty state', () => {
  const { container } = render(<EmptyReadOnly />);
  expect(container).toMatchSnapshot();
});

test('useApps hook returns expected props', async () => {
  const ctx = new Context();
  const acl = makeAcl(sample.acl);
  ctx.isEnterprise = true;
  ctx.storeUser.setState({ acl });
  ctx.appService.fetchApps = () => Promise.resolve({ apps });

  let hook;

  await act(async () => {
    hook = renderHook(() => useApps(ctx));
  });

  expect(hook.current).toEqual(expect.objectContaining(expectedHookResponse));
});

const sample = {
  acl: {
    tokens: {
      create: true,
    },
    apps: {
      list: true,
      create: true,
      remove: true,
      edit: true,
      read: true,
    },
  },
};

const expectedHookResponse = {
  clusterId: 'im-a-cluster',
  isLeafCluster: false,
  isEnterprise: true,
  isAddAppVisible: false,
  canCreate: true,
  attempt: { status: 'success' },
  apps,
};
