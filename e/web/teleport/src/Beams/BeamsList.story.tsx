import { Meta, StoryObj } from '@storybook/react-vite';
import { QueryClient } from '@tanstack/react-query';
import { Route, Routes } from 'react-router';

import cfg from 'e-teleport/config';
import {
  BeamsProviders,
  defaultBeams,
  listBeamsError,
  listBeamsForever,
  listBeamsSuccess,
} from 'e-teleport/test/helpers/beams';
import { defaultAccess, makeAcl } from 'teleport/services/user/makeAcl';

import { BeamsList } from './BeamsList';

const queryClient = new QueryClient({
  defaultOptions: {
    queries: {
      refetchOnWindowFocus: false,
      retry: false,
    },
  },
});

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
  beforeEach({ msw }) {
    msw.use(listBeamsSuccess());
  },
};

const ownedBeams = defaultBeams.map(b => ({ ...b, user: 'llama' }));

export const WithFullAccess: Story = {
  args: { hasChangePermission: true },

  beforeEach({ msw }) {
    msw.use(listBeamsSuccess({ items: ownedBeams, next_page_token: '' }));
  },
};

export const AdminViewingOthers: Story = {
  args: { hasChangePermission: true, showMyBeamsOnly: false },

  beforeEach({ msw }) {
    msw.use(listBeamsSuccess());
  },
};

export const Empty: Story = {
  beforeEach({ msw }) {
    msw.use(
      listBeamsSuccess({
        items: [],
        next_page_token: '',
      })
    );
  },
};

export const NoListPermission: Story = {
  args: { hasListPermission: false },
};

export const Error: Story = {
  beforeEach({ msw }) {
    msw.use(listBeamsError(500, 'something went wrong'));
  },
};

export const OutdatedProxy: Story = {
  beforeEach({ msw }) {
    msw.use(
      listBeamsError(404, 'path not found', {
        proxyVersion: {
          major: 18,
          minor: 0,
          patch: 0,
          preRelease: '',
          string: '18.0.0',
        },
      })
    );
  },
};

export const UnsupportedSort: Story = {
  beforeEach({ msw }) {
    msw.use(listBeamsError(400, 'unsupported sort, with some more info'));
  },
};

export const Loading: Story = {
  beforeEach({ msw }) {
    msw.use(listBeamsForever());
  },
};

function Wrapper(props?: {
  hasListPermission?: boolean;
  hasChangePermission?: boolean;
  showMyBeamsOnly?: boolean;
}) {
  const {
    hasListPermission = true,
    hasChangePermission = false,
    showMyBeamsOnly = true,
  } = props ?? {};

  const acl = makeAcl({
    beam: {
      ...defaultAccess,
      list: hasListPermission,
      read: hasListPermission,
      create: hasChangePermission,
      edit: hasChangePermission,
      remove: hasChangePermission,
    },
  });

  const initialUrl = showMyBeamsOnly
    ? cfg.routes.beamsList
    : `${cfg.routes.beamsList}?own=false`;

  return (
    <BeamsProviders
      acl={acl}
      queryClient={queryClient}
      initialEntries={[initialUrl]}
    >
      <Routes>
        <Route path={cfg.routes.beamsList} element={<BeamsList />} />
      </Routes>
    </BeamsProviders>
  );
}
