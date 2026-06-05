import { Meta, StoryObj } from '@storybook/react-vite';
import { QueryClient, QueryClientProvider } from '@tanstack/react-query';
import { Route, Routes } from 'react-router';

import cfg from 'e-teleport/config';
import {
  listBeamsError,
  listBeamsForever,
  listBeamsSuccess,
} from 'e-teleport/test/helpers/beams';
import { createTeleportContext } from 'teleport/mocks/contexts';
import { TeleportProviderBasic } from 'teleport/mocks/providers';
import { defaultAccess, makeAcl } from 'teleport/services/user/makeAcl';

import { BeamsList } from './BeamsList';

const meta = {
  title: 'TeleportE/Beams/List',
  component: Wrapper,
  beforeEach: () => {
    queryClient.clear(); // Prevent cached data sharing between stories
  },
} satisfies Meta<typeof Wrapper>;

type Story = StoryObj<typeof meta>;

export default meta;

export const Happy: Story = {
  parameters: {
    msw: {
      handlers: [listBeamsSuccess()],
    },
  },
};

export const Empty: Story = {
  parameters: {
    msw: {
      handlers: [
        listBeamsSuccess({
          items: [],
          next_page_token: null,
        }),
      ],
    },
  },
};

export const NoListPermission: Story = {
  args: { hasListPermission: false },
  parameters: {
    msw: {
      handlers: [
        /* should never make a call */
      ],
    },
  },
};

export const Error: Story = {
  parameters: {
    msw: {
      handlers: [listBeamsError(500, 'something went wrong')],
    },
  },
};

export const OutdatedProxy: Story = {
  parameters: {
    msw: {
      handlers: [
        listBeamsError(404, 'path not found', {
          proxyVersion: {
            major: 18,
            minor: 0,
            patch: 0,
            preRelease: '',
            string: '18.0.0',
          },
        }),
      ],
    },
  },
};

export const UnsupportedSort: Story = {
  parameters: {
    msw: {
      handlers: [listBeamsError(400, 'unsupported sort, with some more info')],
    },
  },
};

export const Loading: Story = {
  parameters: {
    msw: {
      handlers: [listBeamsForever()],
    },
  },
};

const queryClient = new QueryClient({
  defaultOptions: {
    queries: {
      refetchOnWindowFocus: false,
      retry: false,
    },
  },
});

function Wrapper(props?: { hasListPermission?: boolean }) {
  const { hasListPermission = true } = props ?? {};

  const customAcl = makeAcl({
    beam: {
      ...defaultAccess,
      list: hasListPermission,
      read: hasListPermission,
    },
  });

  const ctx = createTeleportContext({
    customAcl,
  });

  return (
    <QueryClientProvider client={queryClient}>
      <TeleportProviderBasic
        teleportCtx={ctx}
        initialEntries={[cfg.routes.beamsList]}
      >
        <Routes>
          <Route path={cfg.routes.beamsList} element={<BeamsList />} />
        </Routes>
      </TeleportProviderBasic>
    </QueryClientProvider>
  );
}
