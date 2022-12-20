import React from 'react';
import { screen } from '@testing-library/react';
import { fireEvent, render } from 'design/utils/testing';

import { MockAppContextProvider } from 'teleterm/ui/fixtures/MockAppContextProvider';
import { MockAppContext } from 'teleterm/ui/fixtures/mocks';
import { IAppContext } from 'teleterm/ui/types';
import { Cluster } from 'teleterm/services/tshd/types';

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
  jest
    .spyOn(appContext.clustersService, 'findCluster')
    .mockImplementation(() => {
      return {
        loggedInUser: { name: 'alice' },
      } as Cluster;
    });

  jest
    .spyOn(appContext.workspacesService, 'getRootClusterUri')
    .mockReturnValue(clusterUri);

  renderOpenedShareFeedback(appContext);

  expect(appContext.clustersService.findCluster).toHaveBeenCalledWith(
    clusterUri
  );
  expect(screen.getByLabelText('Email Address')).toHaveValue('');
});

test('email field is prefilled with the username if it looks like an email', () => {
  const appContext = new MockAppContext();
  const clusterUri = '/clusters/production';
  jest
    .spyOn(appContext.clustersService, 'findCluster')
    .mockImplementation(() => {
      return {
        loggedInUser: {
          name: 'bob@prod.com',
        },
      } as Cluster;
    });

  jest
    .spyOn(appContext.workspacesService, 'getRootClusterUri')
    .mockReturnValue(clusterUri);

  renderOpenedShareFeedback(appContext);

  expect(appContext.clustersService.findCluster).toHaveBeenCalledWith(
    clusterUri
  );
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
