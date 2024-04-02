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
import { render, screen } from 'design/utils/testing';
import userEvent from '@testing-library/user-event';

import {
  Integration,
  IntegrationKind,
  IntegrationStatusCode,
} from 'teleport/services/integrations';

import { EditAwsOidcIntegrationDialog } from './EditAwsOidcIntegrationDialog';

test('user acknowledging script was ran when s3 bucket fields are edited', async () => {
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
          issuerS3Bucket: 'test-value',
          issuerS3Prefix: '',
        },
        statusCode: IntegrationStatusCode.Running,
      }}
    />
  );

  // Initial state.
  expect(screen.queryByTestId('scriptbox')).not.toBeInTheDocument();
  expect(screen.queryByTestId('checkbox')).not.toBeInTheDocument();
  expect(
    screen.queryByRole('button', { name: /generate command/i })
  ).not.toBeInTheDocument();
  expect(screen.getByRole('button', { name: /save/i })).toBeDisabled();

  // Fill in the s3 prefix field.
  await userEvent.type(screen.getByPlaceholderText(/prefix/i), 'test-value');

  // When clicking on generate command:
  //  - script rendered
  //  - checkbox to confirm user has ran command
  //  - edit button replaces generate command button
  //  - save button still disabled
  await userEvent.click(
    screen.getByRole('button', { name: /generate command/i })
  );
  screen.getByRole('button', { name: /edit/i });
  expect(screen.getByRole('button', { name: /save/i })).toBeDisabled();
  expect(
    screen.queryByRole('button', { name: /generate command/i })
  ).not.toBeInTheDocument();
  expect(screen.getByTestId('checkbox')).toBeInTheDocument();
  expect(screen.getByTestId('scriptbox')).toBeInTheDocument();

  // Click on checkbox should enable save button and disable edit button.
  await userEvent.click(screen.getByRole('checkbox'));
  expect(screen.getByRole('button', { name: /save/i })).toBeEnabled();
  expect(screen.getByRole('button', { name: /edit/i })).toBeDisabled();

  // Unchecking the checkbox should disable save button.
  await userEvent.click(screen.getByRole('checkbox'));
  expect(screen.getByRole('button', { name: /save/i })).toBeDisabled();

  // Click on edit, should replace it with generate command
  await userEvent.click(screen.getByRole('button', { name: /edit/i }));
  expect(
    screen.getByRole('button', { name: /generate command/i })
  ).toBeEnabled();
});

test('render warning on save when leaving s3 fields empty', async () => {
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
          issuerS3Bucket: '',
          issuerS3Prefix: '',
        },
        statusCode: IntegrationStatusCode.Running,
      }}
    />
  );

  // Initial state.
  expect(screen.queryByTestId('scriptbox')).not.toBeInTheDocument();
  expect(screen.queryByTestId('checkbox')).not.toBeInTheDocument();
  expect(screen.getByRole('button', { name: /save/i })).toBeDisabled();
  expect(
    screen.queryByRole('button', { name: /generate command/i })
  ).not.toBeInTheDocument();

  // Enable the save button by changing a field.
  await userEvent.type(
    screen.getByPlaceholderText(/arn:aws:iam:/i),
    'arn:aws:iam::123456789012:role/someonelse'
  );

  // Clicking on save without defining s3 fields, should render
  // a warning.
  await userEvent.click(screen.getByRole('button', { name: /save/i }));
  expect(edit).not.toHaveBeenCalled();
  expect(
    screen.getByText(/recommended to use an S3 bucket/i)
  ).toBeInTheDocument();

  // Canceling and saving should re-render the warning.
  await userEvent.click(screen.getByRole('button', { name: /cancel/i }));
  await userEvent.click(screen.getByRole('button', { name: /save/i }));

  await userEvent.click(screen.getByRole('button', { name: /continue/i }));
  expect(edit).toHaveBeenCalledTimes(1);
});

test('render warning on save when deleting existing s3 fields', async () => {
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
          issuerS3Bucket: 'delete-me',
          issuerS3Prefix: 'delete-me',
        },
        statusCode: IntegrationStatusCode.Running,
      }}
    />
  );

  expect(
    screen.queryByRole('button', { name: /generate command/i })
  ).not.toBeInTheDocument();

  // Delete the s3 fields.
  await userEvent.clear(screen.getByPlaceholderText(/bucket/i));
  await userEvent.clear(screen.getByPlaceholderText(/prefix/i));

  expect(
    screen.queryByRole('button', { name: /generate command/i })
  ).not.toBeInTheDocument();

  // Test for warning render.
  await userEvent.click(screen.getByRole('button', { name: /save/i }));
  expect(edit).not.toHaveBeenCalled();
  expect(
    screen.getByText(/recommended to use an S3 bucket/i)
  ).toBeInTheDocument();

  await userEvent.click(screen.getByRole('button', { name: /continue/i }));
  expect(edit).toHaveBeenCalledTimes(1);
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
  await userEvent.type(
    screen.getByPlaceholderText(/arn:aws:iam:/i),
    'role something else'
  );

  await userEvent.click(screen.getByRole('button', { name: /save/i }));
  expect(screen.getByText(/invalid role ARN format/i)).toBeInTheDocument();
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
  await userEvent.clear(screen.getByPlaceholderText(/arn:aws:iam:/i));
  await userEvent.type(
    screen.getByPlaceholderText(/arn:aws:iam:/i),
    'arn:aws:iam::123456789011:role/other'
  );

  // change s3 fields
  await userEvent.clear(screen.getByPlaceholderText(/bucket/i));
  await userEvent.type(screen.getByPlaceholderText(/bucket/i), 'other-bucket');
  await userEvent.clear(screen.getByPlaceholderText(/prefix/i));
  await userEvent.type(screen.getByPlaceholderText(/prefix/i), 'other-prefix');

  await userEvent.click(
    screen.getByRole('button', { name: /generate command/i })
  );
  await userEvent.click(screen.getByRole('checkbox'));
  await userEvent.click(screen.getByRole('button', { name: /save/i }));

  expect(mockEditFn).toHaveBeenCalledWith({
    roleArn: 'arn:aws:iam::123456789011:role/other',
    s3Bucket: 'other-bucket',
    s3Prefix: 'other-prefix',
  });
});

const integration: Integration = {
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
