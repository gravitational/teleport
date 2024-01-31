import { render, screen, userEvent, fireEvent } from 'design/utils/testing';

import { RequestCheckout, RequestCheckoutProps } from './RequestCheckout';

test('start with no suggested reviewers', async () => {
  render(<RequestCheckout {...props} reviewers={[]} />);

  // Test init renders no reviewers.
  let reviewers = screen.getByTestId('reviewers');
  expect(reviewers.childNodes).toHaveLength(0);

  // Add a reviewer
  await userEvent.click(screen.getByRole('button', { name: 'Add' }));
  await userEvent.type(
    screen.getByText(/type or select a name/i),
    'llama{enter}'
  );
  await userEvent.click(screen.getByRole('button', { name: 'Done' }));

  reviewers = screen.getByTestId('reviewers');
  expect(reviewers.childNodes).toHaveLength(1);
  expect(reviewers.childNodes[0]).toHaveTextContent('llama');

  // Remove by clicking on x button.
  fireEvent.click(reviewers.childNodes[0].lastChild);
  reviewers = screen.getByTestId('reviewers');
  expect(reviewers.childNodes).toHaveLength(0);
});

test('start with suggested reviewers', async () => {
  render(<RequestCheckout {...props} reviewers={['llama']} />);

  // Test init renders reviewers.
  let reviewers = screen.getByTestId('reviewers');
  expect(reviewers.childNodes).toHaveLength(1);
  expect(reviewers.childNodes[0]).toHaveTextContent('llama');

  // Add another reviewer.
  await userEvent.click(screen.getByRole('button', { name: 'Edit' }));
  await userEvent.type(
    screen.getByText(/type or select a name/i),
    'alpaca{enter}'
  );
  await userEvent.click(screen.getByRole('button', { name: 'Done' }));

  reviewers = screen.getByTestId('reviewers');
  expect(reviewers.childNodes).toHaveLength(2);
  expect(reviewers.childNodes[0]).toHaveTextContent('llama');
  expect(reviewers.childNodes[1]).toHaveTextContent('alpaca');

  // Remove a suggested reviewer by typing the name.
  await userEvent.click(screen.getByRole('button', { name: 'Edit' }));
  await userEvent.type(
    screen.getByText(/type or select a name/i),
    'llama{enter}'
  );
  await userEvent.click(screen.getByRole('button', { name: 'Done' }));

  reviewers = screen.getByTestId('reviewers');
  expect(reviewers.childNodes).toHaveLength(1);
  expect(reviewers.childNodes[0]).toHaveTextContent('alpaca');

  // Suggested reviewer should still be rendered in the dropdown.
  await userEvent.click(screen.getByRole('button', { name: 'Edit' }));
  await userEvent.click(screen.getByTitle(/llama/i));
  await userEvent.click(screen.getByRole('button', { name: 'Done' }));

  reviewers = screen.getByTestId('reviewers');
  expect(reviewers.childNodes).toHaveLength(2);
  expect(reviewers.childNodes[0]).toHaveTextContent('alpaca');
  expect(reviewers.childNodes[1]).toHaveTextContent('llama');
});

const props: RequestCheckoutProps = {
  createAttempt: { status: '' },
  fetchResourceRequestRolesAttempt: { status: '' },
  isResourceRequest: false,
  requireReason: true,
  reviewers: ['llama', 'alpaca'],
  createRequest: () => null,
  data: [],
  clearAttempt: () => null,
  onClose: () => null,
  toggleResource: () => null,
  reset: () => null,
  transitionState: 'entered',
  numRequestedResources: 4,
  resourceRequestRoles: ['admin', 'access', 'developer'],
  selectedResourceRequestRoles: ['admin', 'access'],
  setSelectedResourceRequestRoles: () => null,
  fetchStatus: 'loaded',
  durationOptions: [{ value: 0, label: '' }],
  maxDuration: { value: 0, label: '12 hours' },
  setMaxDuration: () => null,
  requestTTLDurationOptions: [{ value: 0, label: '' }],
  requestTTL: { value: 0, label: '1 hour' },
  setRequestTTL: () => null,
};
