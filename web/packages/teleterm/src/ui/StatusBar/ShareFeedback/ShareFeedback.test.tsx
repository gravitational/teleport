/**
 * Teleport
 * Copyright (C) 2023  Gravitational, Inc.
 *
 * This program is free software: you can redistribute it and/or modify
 * it under the terms of the GNU Affero General Public License as published by
 * the Free Software Foundation, either version 3 of the License, or
 * (at your option) any later version.
 *
 * This program is distributed in the hope that it will be useful,
 * but WITHOUT ANY WARRANTY; without even the implied warranty of
 * MERCHANTABILITY or FITNESS FOR A PARTICULAR PURPOSE.  See the
 * GNU Affero General Public License for more details.
 *
 * You should have received a copy of the GNU Affero General Public License
 * along with this program.  If not, see <http://www.gnu.org/licenses/>.
 */

import { screen } from '@testing-library/react';

import { fireEvent, render } from 'design/utils/testing';

import {
  makeLoggedInUser,
  makeRootCluster,
} from 'teleterm/services/tshd/testHelpers';
import { MockAppContextProvider } from 'teleterm/ui/fixtures/MockAppContextProvider';
import { MockAppContext } from 'teleterm/ui/fixtures/mocks';
import { IAppContext } from 'teleterm/ui/types';

import { ShareFeedback } from './ShareFeedback';

function renderOpenedShareFeedback(appContext: IAppContext) {
  const utils = render(
    <MockAppContextProvider appContext={appContext}>
      <ShareFeedback />
    </MockAppContextProvider>
  );

  fireEvent.click(screen.getByTitle('Share feedback'));
  return utils;
}

test('email field is not prefilled with the username if is not an email', () => {
  const appContext = new MockAppContext();
  const clusterUri = '/clusters/localhost';
  appContext.workspacesService.setState(draft => {
    draft.rootClusterUri = clusterUri;
  });
  appContext.clustersService.setState(draft => {
    draft.clusters.set(
      clusterUri,
      makeRootCluster({
        uri: clusterUri,
        loggedInUser: makeLoggedInUser({ name: 'alice' }),
      })
    );
  });

  renderOpenedShareFeedback(appContext);

  expect(screen.getByLabelText('Email Address')).toHaveValue('');
});

test('email field is prefilled with the username if it looks like an email', () => {
  const appContext = new MockAppContext();
  const clusterUri = '/clusters/production';
  appContext.workspacesService.setState(draft => {
    draft.rootClusterUri = clusterUri;
  });
  appContext.clustersService.setState(draft => {
    draft.clusters.set(
      clusterUri,
      makeRootCluster({
        uri: clusterUri,
        loggedInUser: makeLoggedInUser({ name: 'bob@prod.com' }),
      })
    );
  });

  renderOpenedShareFeedback(appContext);

  expect(screen.getByLabelText('Email Address')).toHaveValue('bob@prod.com');
});

test('element is hidden after clicking close button', () => {
  const appContext = new MockAppContext();

  renderOpenedShareFeedback(appContext);

  fireEvent.click(screen.getByTitle('Close'));

  expect(
    screen.queryByTestId('share-feedback-container')
  ).not.toBeInTheDocument();
});
