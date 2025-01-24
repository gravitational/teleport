import { MemoryRouter } from 'react-router';

import { render, screen } from 'design/utils/testing';

import { createTeleportContextE } from 'e-teleport/mocks/contexts';
import { ContextProvider } from 'teleport/index';
import { makeTestUserContext } from 'teleport/User/testHelpers/makeTestUserContext';
import { mockUserContextProviderWith } from 'teleport/User/testHelpers/mockUserContextWith';

import { AccessGraph } from './AccessGraph';

test('renders the empty state if user has no permissions', async () => {
  mockUserContextProviderWith(makeTestUserContext());
  const ctx = createTeleportContextE();

  render(
    <MemoryRouter>
      <ContextProvider ctx={ctx}>
        <AccessGraph />
      </ContextProvider>
    </MemoryRouter>
  );

  expect(screen.getByTestId('tag-empty-state')).toBeInTheDocument();
});
