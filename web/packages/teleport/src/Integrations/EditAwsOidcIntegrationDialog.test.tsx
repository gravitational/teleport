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
import userEvent from '@testing-library/user-event';
import { useEffect } from 'react';
import { MemoryRouter } from 'react-router';

import { fireEvent, render, screen, waitFor } from 'design/utils/testing';

import cfg from 'teleport/config';
import {
  IntegrationAwsOidc,
  IntegrationKind,
  integrationService,
  IntegrationStatusCode,
} from 'teleport/services/integrations';

import { EditAwsOidcIntegrationDialog } from './EditAwsOidcIntegrationDialog';
import { useIntegrationOperation } from './Operations';

test('user acknowledging script was ran when reconfiguring', async () => {
  render(
    <EditAwsOidcIntegrationDialog
      close={() => null}
      edit={() => null}
      integration={{
        resourceType: 'integration',
        kind: IntegrationKind.AwsOidc,
        name: 'some-integration-name',
        spec: {
          roleArn: 'arn:aws:iam::123456789012:role/johndoe',
        },
        statusCode: IntegrationStatusCode.Running,
      }}
    />
  );

  // Initial state.
  expect(screen.queryByTestId('scriptbox')).not.toBeInTheDocument();
  expect(screen.queryByLabelText(/I ran the command/i)).not.toBeInTheDocument();
  expect(
    screen.queryByRole('button', { name: /reconfigure/i })
  ).not.toBeInTheDocument();
  expect(screen.getByRole('button', { name: /save/i })).toBeDisabled();

  // Check s3 related fields are not rendered.
  expect(screen.queryByText(/not recommended/)).not.toBeInTheDocument();
  expect(screen.queryByText('Amazon S3')).not.toBeInTheDocument();

  // change role arn
  fireEvent.change(screen.getByPlaceholderText(/arn:aws:iam:/i), {
    target: { value: 'arn:aws:iam::123456789011:role/other' },
  });

  await waitFor(() =>
    expect(screen.getByRole('button', { name: /reconfigure/i })).toBeEnabled()
  );
  // When clicking on reconfigure:
  //  - script rendered
  //  - checkbox to confirm user has ran command
  //  - edit button replaces reconfigure button
  //  - save button still disabled
  await userEvent.click(screen.getByRole('button', { name: /reconfigure/i }));
  await screen.findByRole('button', { name: /edit/i });
  expect(screen.getByRole('button', { name: /save/i })).toBeDisabled();
  expect(
    screen.queryByRole('button', { name: /reconfigure/i })
  ).not.toBeInTheDocument();
  expect(screen.getByLabelText(/I ran the command/i)).toBeInTheDocument();
  expect(screen.getByTestId('scriptbox')).toBeInTheDocument();

  // Click on checkbox should enable save button and disable edit button.
  await userEvent.click(screen.getByRole('checkbox'));
  await waitFor(() =>
    expect(screen.getByRole('button', { name: /save/i })).toBeEnabled()
  );
  expect(screen.getByRole('button', { name: /edit/i })).toBeDisabled();

  // Unchecking the checkbox should disable save button.
  await userEvent.click(screen.getByRole('checkbox'));
  await waitFor(() =>
    expect(screen.getByRole('button', { name: /save/i })).toBeDisabled()
  );

  // Click on edit, should replace it with reconfigure
  await userEvent.click(screen.getByRole('button', { name: /edit/i }));
  await waitFor(() =>
    expect(screen.getByRole('button', { name: /reconfigure/i })).toBeEnabled()
  );
});

test('health check is called before calling update', async () => {
  const spyPing = jest
    .spyOn(integrationService, 'pingAwsOidcIntegration')
    .mockResolvedValue({} as any); // response doesn't matter

  const spyUpdate = jest
    .spyOn(integrationService, 'updateIntegration')
    .mockResolvedValue({} as any); // response doesn't matter

  render(
    <MemoryRouter initialEntries={[cfg.getClusterRoute('some-cluster')]}>
      <ComponentWithEditOperation />
    </MemoryRouter>
  );

  // change role arn
  fireEvent.change(screen.getByPlaceholderText(/arn:aws:iam:/i), {
    target: { value: 'arn:aws:iam::123456789011:role/other' },
  });

  await waitFor(() =>
    expect(screen.getByRole('button', { name: /reconfigure/i })).toBeEnabled()
  );
  await userEvent.click(screen.getByRole('button', { name: /reconfigure/i }));

  // Click on checkbox to enable save button.
  await userEvent.click(screen.getByRole('checkbox'));
  await waitFor(() =>
    expect(screen.getByRole('button', { name: /save/i })).toBeEnabled()
  );
  await userEvent.click(screen.getByRole('button', { name: /save/i }));

  await waitFor(() => expect(spyPing).toHaveBeenCalledTimes(1));
  await waitFor(() => expect(spyUpdate).toHaveBeenCalledTimes(1));

  const pingOrder = spyPing.mock.invocationCallOrder[0];
  const createOrder = spyUpdate.mock.invocationCallOrder[0];
  expect(pingOrder).toBeLessThan(createOrder);
});

test('render warning when s3 buckets are present', async () => {
  const edit = jest.fn(() => Promise.resolve());
  render(
    <EditAwsOidcIntegrationDialog
      close={() => null}
      edit={edit}
      integration={{
        resourceType: 'integration',
        kind: IntegrationKind.AwsOidc,
        name: 'some-integration-name',
        spec: {
          roleArn: 'arn:aws:iam::123456789012:role/johndoe',
          issuerS3Bucket: 'some-bucket',
          issuerS3Prefix: 'some-prefix',
        },
        statusCode: IntegrationStatusCode.Running,
      }}
    />
  );

  // Initial state.
  expect(screen.queryByTestId('scriptbox')).not.toBeInTheDocument();
  expect(screen.queryByLabelText(/I ran the command/i)).not.toBeInTheDocument();
  expect(screen.getByRole('button', { name: /save/i })).toBeDisabled();

  // Check s3 related fields/warnings are rendered.
  expect(
    screen.getByRole('button', { name: /reconfigure/i })
  ).toBeInTheDocument();
  expect(screen.getByText(/not recommended/)).toBeInTheDocument();
  expect(screen.getByText(/Amazon S3 Location/)).toBeInTheDocument();

  // Clicking on reconfigure should hide s3 fields.
  await userEvent.click(screen.getByRole('button', { name: /reconfigure/i }));
  await screen.findByText(/AWS CloudShell/);
  expect(screen.queryByText(/not recommended/)).not.toBeInTheDocument();
  expect(screen.queryByText('/Amazon S3 Location/')).not.toBeInTheDocument();

  // Clicking on edit, should render it back.
  await userEvent.click(screen.getByRole('button', { name: /edit/i }));

  await screen.findByText(/not recommended/);
  await screen.findByText(/Amazon S3 Location/);
});

test('edit invalid fields', async () => {
  render(
    <EditAwsOidcIntegrationDialog
      close={() => null}
      edit={() => null}
      integration={integration}
    />
  );

  expect(screen.getByRole('button', { name: /save/i })).toBeDisabled();

  // invalid role arn
  fireEvent.change(screen.getByPlaceholderText(/arn:aws:iam:/i), {
    target: { value: 'role something else' },
  });

  await waitFor(() =>
    expect(screen.getByRole('button', { name: /reconfigure/i })).toBeEnabled()
  );

  await userEvent.click(screen.getByRole('button', { name: /reconfigure/i }));
  await screen.findByText(/invalid role ARN format/i);
});

test('edit submit called with proper fields', async () => {
  const mockEditFn = jest.fn(() => Promise.resolve());
  render(
    <EditAwsOidcIntegrationDialog
      close={() => null}
      edit={mockEditFn}
      integration={integration}
    />
  );

  expect(screen.getByRole('button', { name: /save/i })).toBeDisabled();

  // change role arn
  fireEvent.change(screen.getByPlaceholderText(/arn:aws:iam:/i), {
    target: { value: 'arn:aws:iam::123456789011:role/other' },
  });

  // change s3 fields
  fireEvent.change(screen.getByPlaceholderText(/bucket/i), {
    target: { value: 'other-bucket' },
  });
  fireEvent.change(screen.getByPlaceholderText(/prefix/i), {
    target: { value: 'other-prefix' },
  });

  await waitFor(() =>
    expect(screen.getByRole('button', { name: /reconfigure/i })).toBeEnabled()
  );

  await userEvent.click(screen.getByRole('button', { name: /reconfigure/i }));
  await screen.findByRole('button', { name: /edit/i });

  await userEvent.click(screen.getByLabelText(/I ran the command/i));
  await waitFor(() =>
    expect(screen.getByRole('button', { name: /save/i })).toBeEnabled()
  );
  await userEvent.click(screen.getByRole('button', { name: /save/i }));
  await waitFor(() => expect(mockEditFn).toHaveBeenCalledTimes(1));

  expect(mockEditFn).toHaveBeenCalledWith({
    kind: IntegrationKind.AwsOidc,
    roleArn: 'arn:aws:iam::123456789011:role/other',
  });
});

const integration: IntegrationAwsOidc = {
  resourceType: 'integration',
  kind: IntegrationKind.AwsOidc,
  name: 'some-integration-name',
  spec: {
    roleArn: 'arn:aws:iam::123456789012:role/johndoe',
    issuerS3Bucket: 's3-bucket',
    issuerS3Prefix: 's3-prefix',
  },
  statusCode: IntegrationStatusCode.Running,
};

function ComponentWithEditOperation() {
  const integrationOps = useIntegrationOperation();
  useEffect(() => {
    integrationOps.onEdit(integration);
  }, []);

  return (
    <EditAwsOidcIntegrationDialog
      close={() => null}
      edit={req => integrationOps.edit(req).then()}
      integration={integration}
    />
  );
}
