/**
 * Teleport
 * Copyright (C) 2025  Gravitational, Inc.
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

import { createMemoryHistory } from 'history';
import type { PropsWithChildren } from 'react';
import { Router } from 'react-router';

import { Providers, render, screen } from 'design/utils/testing';

import { type BaseIntegration } from './common';
import { IntegrationPicker } from './IntegrationPicker';

type TestIntegration = BaseIntegration & {
  type: string;
};

const mockIntegrations: TestIntegration[] = [
  {
    name: 'AWS OIDC',
    type: 'aws-oidc',
    tags: ['idp'],
  },
  {
    name: 'Google Cloud Platform',
    type: 'gcp',
    tags: ['resourceaccess'],
  },
  {
    title: 'Jenkins',
    type: 'jenkins',
    tags: ['cicd'],
  },
];

const defaultProps = {
  integrations: mockIntegrations,
  renderIntegration: (i: TestIntegration) => (
    <div key={i.type} data-testid={`integration-${i.type}`}>
      {i.title || i.name}
    </div>
  ),
  canCreate: true,
  initialSort: (a: TestIntegration, b: TestIntegration) => {
    const aName = a.title || a.name;
    const bName = b.title || b.name;
    return aName.localeCompare(bName);
  },
};

function makeWrapper(route: string = '/') {
  const history = createMemoryHistory({
    initialEntries: [route],
  });

  return function wrapper({ children }: PropsWithChildren) {
    return (
      <Providers>
        <Router history={history}>{children}</Router>
      </Providers>
    );
  };
}

describe('IntegrationPicker', () => {
  beforeEach(() => {
    jest.clearAllMocks();
  });

  describe('filtering', () => {
    test('filters integrations by search term', () => {
      render(<IntegrationPicker {...defaultProps} />, {
        wrapper: makeWrapper('/?search=aws'),
      });

      expect(screen.getByTestId('integration-aws-oidc')).toBeInTheDocument();
      expect(screen.queryByTestId('integration-gcp')).not.toBeInTheDocument();
      expect(
        screen.queryByTestId('integration-jenkins')
      ).not.toBeInTheDocument();
    });

    test('filters integrations by search term matches tags', () => {
      render(<IntegrationPicker {...defaultProps} />, {
        wrapper: makeWrapper('/?search=cicd'),
      });

      expect(screen.getByTestId('integration-jenkins')).toBeInTheDocument();
      expect(
        screen.queryByTestId('integration-aws-oidc')
      ).not.toBeInTheDocument();
      expect(screen.queryByTestId('integration-gcp')).not.toBeInTheDocument();
    });

    test('filters integrations by multiple search terms', () => {
      render(<IntegrationPicker {...defaultProps} />, {
        wrapper: makeWrapper('/?search=aws%20oidc'),
      });

      expect(screen.getByTestId('integration-aws-oidc')).toBeInTheDocument();
      expect(screen.queryByTestId('integration-gcp')).not.toBeInTheDocument();
      expect(
        screen.queryByTestId('integration-jenkins')
      ).not.toBeInTheDocument();
    });

    test('filters integrations by tags', () => {
      render(<IntegrationPicker {...defaultProps} />, {
        wrapper: makeWrapper('/?tags=cicd'),
      });

      expect(screen.getByTestId('integration-jenkins')).toBeInTheDocument();
      expect(
        screen.queryByTestId('integration-aws-oidc')
      ).not.toBeInTheDocument();
      expect(screen.queryByTestId('integration-gcp')).not.toBeInTheDocument();
    });

    test('filters integrations by multiple tags', () => {
      render(<IntegrationPicker {...defaultProps} />, {
        wrapper: makeWrapper('/?tags=idp&tags=cicd'),
      });

      expect(screen.getByTestId('integration-aws-oidc')).toBeInTheDocument();
      expect(screen.getByTestId('integration-jenkins')).toBeInTheDocument();
      expect(screen.queryByTestId('integration-gcp')).not.toBeInTheDocument();
    });

    test('combines search and tag filters', () => {
      render(<IntegrationPicker {...defaultProps} />, {
        wrapper: makeWrapper('/?search=jEnKiNs&tags=cicd'),
      });

      expect(screen.getByTestId('integration-jenkins')).toBeInTheDocument();
      expect(
        screen.queryByTestId('integration-aws-oidc')
      ).not.toBeInTheDocument();
      expect(screen.queryByTestId('integration-gcp')).not.toBeInTheDocument();
    });

    test('shows no results when nothing matches', () => {
      render(<IntegrationPicker {...defaultProps} />, {
        wrapper: makeWrapper('/?search=netbsd'),
      });

      expect(screen.getByText('No results found')).toBeInTheDocument();
      expect(
        screen.queryByTestId('integration-aws-oidc')
      ).not.toBeInTheDocument();
      expect(screen.queryByTestId('integration-gcp')).not.toBeInTheDocument();
      expect(
        screen.queryByTestId('integration-jenkins')
      ).not.toBeInTheDocument();
    });

    test('shows all integrations when no filters applied', () => {
      render(<IntegrationPicker {...defaultProps} />, {
        wrapper: makeWrapper(),
      });

      expect(screen.getByTestId('integration-aws-oidc')).toBeInTheDocument();
      expect(screen.getByTestId('integration-gcp')).toBeInTheDocument();
      expect(screen.getByTestId('integration-jenkins')).toBeInTheDocument();
    });
  });

  describe('sorting', () => {
    test('sorts by name in ascending order', () => {
      render(<IntegrationPicker {...defaultProps} />, {
        wrapper: makeWrapper('/?sort=name&direction=ASC'),
      });

      const integrations = screen.getAllByTestId(/integration-/);
      expect(integrations[0]).toHaveTextContent('AWS OIDC');
      expect(integrations[1]).toHaveTextContent('Google Cloud Platform');
      expect(integrations[2]).toHaveTextContent('Jenkins');
    });

    test('sorts by name in descending order', () => {
      render(<IntegrationPicker {...defaultProps} />, {
        wrapper: makeWrapper('/?sort=name&direction=DESC'),
      });

      const integrations = screen.getAllByTestId(/integration-/);
      expect(integrations[0]).toHaveTextContent('Jenkins');
      expect(integrations[1]).toHaveTextContent('Google Cloud Platform');
      expect(integrations[2]).toHaveTextContent('AWS OIDC');
    });

    test('uses initial sort when no sort specified', () => {
      const jenkinsSort = jest.fn((a: TestIntegration, b: TestIntegration) => {
        return a.title === 'Jenkins' ? -1 : b.title === 'Jenkins' ? 1 : 0;
      });
      render(
        <IntegrationPicker {...defaultProps} initialSort={jenkinsSort} />,
        {
          wrapper: makeWrapper(),
        }
      );

      expect(jenkinsSort).toHaveBeenCalled();
      const integrations = screen.getAllByTestId(/integration-/);
      expect(integrations[0]).toHaveTextContent('Jenkins');
    });
  });

  describe('loading and error states', () => {
    test('shows loading indicator when isLoading is true', async () => {
      render(<IntegrationPicker {...defaultProps} isLoading={true} />, {
        wrapper: makeWrapper(),
      });

      expect(await screen.findByTestId('indicator')).toBeInTheDocument();
      expect(
        screen.queryByTestId('integration-aws-oidc')
      ).not.toBeInTheDocument();
    });

    test('shows error message when ErrorMessage is provided', () => {
      const ErrorMessage = <div>Something went wrong</div>;
      render(
        <IntegrationPicker {...defaultProps} ErrorMessage={ErrorMessage} />,
        {
          wrapper: makeWrapper(),
        }
      );

      expect(screen.getByText('Something went wrong')).toBeInTheDocument();
      expect(screen.queryByTestId('integration-1')).not.toBeInTheDocument();
    });
  });

  describe('permission notification', () => {
    test('shows permission notification when canCreate is false', () => {
      render(<IntegrationPicker {...defaultProps} canCreate={false} />, {
        wrapper: makeWrapper(),
      });

      expect(
        screen.getByText(/You do not have permission to create Integrations/)
      ).toBeInTheDocument();
      expect(screen.getByText('plugin.create')).toBeInTheDocument();
      expect(screen.getByText('integration.create')).toBeInTheDocument();
    });

    test('does not show permission alert when canCreate is true', () => {
      render(<IntegrationPicker {...defaultProps} canCreate={true} />, {
        wrapper: makeWrapper(),
      });

      expect(
        screen.queryByText(/You do not have permission to create Integrations/)
      ).not.toBeInTheDocument();
    });
  });
});
