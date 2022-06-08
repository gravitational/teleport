import React from 'react';
import { MemoryRouter } from 'react-router';
import { render, screen, fireEvent, act } from 'design/utils/testing';
import { ContextProvider } from 'teleport';
import TeleportContextE from 'e-teleport/teleportContextE';
import makeUserContext from 'teleport/services/user/makeUserContext';
import NewRequest from './NewRequest';

describe('new request behavior', () => {
  const ctx = new TeleportContextE();
  let Component;

  beforeEach(() => {
    ctx.storeUser.setState({ ...userContext });
    jest.spyOn(ctx.nodeService, 'fetchNodes').mockResolvedValue({
      agents: nodes,
      startKey: '',
      totalCount: nodes.length,
    });

    Component = (
      <MemoryRouter initialEntries={[`web/cluster/cluster-id/requests/new`]}>
        <ContextProvider ctx={ctx}>
          <NewRequest />
        </ContextProvider>
      </MemoryRouter>
    );
  });

  test('add and remove a resource from table', async () => {
    await act(async () => render(Component));

    // Initial render is a roles table.
    let rows = screen.getAllByText(/role-/i);
    expect(rows).toHaveLength(
      userContext.accessCapabilities.requestableRoles.length
    );

    // No resources selected yet.
    let checkoutFooter = screen.getByTestId('checkout-footer');
    expect(checkoutFooter.textContent).toContain('0');
    expect(screen.queryByText(/clear selections/i)).not.toBeInTheDocument();
    expect(screen.getByText(/proceed to request/i)).toBeDisabled();

    // Add a resource.
    rows = screen.getAllByText(/add to request/i);
    expect(rows).toHaveLength(3);
    fireEvent.click(rows[0]);
    checkoutFooter = screen.getByTestId('checkout-footer');
    expect(checkoutFooter.textContent).toContain('1');
    expect(screen.getByText(/proceed to request/i)).not.toBeDisabled();

    // Add another one.
    fireEvent.click(rows[1]);
    checkoutFooter = screen.getByTestId('checkout-footer');
    expect(checkoutFooter.textContent).toContain('2');

    // Remove a resource using remove button.
    rows = screen.getAllByText(/remove/i);
    expect(rows).toHaveLength(2);
    fireEvent.click(rows[0]);
    checkoutFooter = screen.getByTestId('checkout-footer');
    expect(checkoutFooter.textContent).toContain('1');

    // Remove resource by using clear button.
    fireEvent.click(screen.getByText(/clear selections/i));
    checkoutFooter = screen.getByTestId('checkout-footer');
    expect(checkoutFooter.textContent).toContain('0');
    expect(screen.queryByText(/clear selections/i)).not.toBeInTheDocument();
    expect(screen.getByText(/proceed to request/i)).toBeDisabled();
  });

  test('clicking on a resource label constructs predicate query', async () => {
    await act(async () => render(Component));

    // We will use node to test predicate (it will be same for all other agents).
    const inputEl = screen
      .getByTestId('resource-selector')
      .querySelector('input');

    await act(async () =>
      fireEvent.change(inputEl, { target: { value: 'node' } })
    );
    fireEvent.focus(inputEl);
    await act(async () =>
      fireEvent.keyDown(inputEl, { key: 'Enter', keyCode: 13 })
    );

    // Click on a label.
    await act(async () => fireEvent.click(screen.getByText(/test: node1/i)));
    let input = screen.getByPlaceholderText(/search/i);
    expect(input).toHaveValue('labels["test"] == "node1"');

    // Click on another label.
    await act(async () => fireEvent.click(screen.getByText(/test: node2/i)));
    input = screen.getByPlaceholderText(/search/i);
    expect(input).toHaveValue(
      'labels["test"] == "node1" && labels["test"] == "node2"'
    );
  });
});

const userContext = makeUserContext({
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
  accessCapabilities: {
    requestableRoles: ['role-1', 'role-2', 'role-3'],
    suggestedReviewers: ['reviewer1', 'reviewer2'],
  },
});

const nodes = [
  {
    tunnel: false,
    id: '1',
    clusterId: 'one',
    hostname: 'node1',
    addr: 'node1-addr',
    labels: [
      {
        name: 'test',
        value: 'node1',
      },
    ],
  },
  {
    tunnel: true,
    id: '2',
    clusterId: 'one',
    hostname: 'node2',
    addr: 'node2-addr',
    labels: [
      {
        name: 'test',
        value: 'node2',
      },
    ],
  },
];
