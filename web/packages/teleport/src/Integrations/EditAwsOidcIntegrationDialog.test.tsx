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
import { render, screen, fireEvent } from 'design/utils/testing';

import {
  Integration,
  IntegrationKind,
  IntegrationStatusCode,
} from 'teleport/services/integrations';

import { EditAwsOidcIntegrationDialog } from './EditAwsOidcIntegrationDialog';

test('edit without s3 fields', async () => {
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
          issuerS3Bucket: '',
          issuerS3Prefix: '',
        },
        statusCode: IntegrationStatusCode.Running,
      }}
    />
  );

  // Initial state.
  expect(screen.getByText(/required/i)).toBeInTheDocument();
  expect(screen.queryByTestId('scriptbox')).not.toBeInTheDocument();
  expect(screen.queryByTestId('checkbox')).not.toBeInTheDocument();
  expect(screen.getByRole('button', { name: /save/i })).toBeDisabled();

  // Click on generate command:
  //  - script rendered
  //  - checkbox to confirm user has ran command
  //  - edit button replaces generate command button
  //  - save button still disabled
  fireEvent.click(screen.getByRole('button', { name: /generate command/i }));
  screen.getByRole('button', { name: /edit/i });
  expect(screen.getByRole('button', { name: /save/i })).toBeDisabled();
  expect(
    screen.queryByRole('button', { name: /generate command/i })
  ).not.toBeInTheDocument();
  expect(screen.getByTestId('checkbox')).toBeInTheDocument();
  expect(screen.getByTestId('scriptbox')).toBeInTheDocument();

  // Click on checkbox should enable save button and disable edit button.
  fireEvent.click(screen.getByRole('checkbox'));
  expect(screen.getByRole('button', { name: /save/i })).toBeEnabled();
  expect(screen.getByRole('button', { name: /edit/i })).toBeDisabled();

  // Unchecking the checkbox should disable save button.
  fireEvent.click(screen.getByRole('checkbox'));
  expect(screen.getByRole('button', { name: /save/i })).toBeDisabled();

  // Click on edit, should replace it with generate command
  fireEvent.click(screen.getByRole('button', { name: /edit/i }));
  expect(
    screen.getByRole('button', { name: /generate command/i })
  ).toBeEnabled();
});

test('edit with s3 fields', async () => {
  render(
    <EditAwsOidcIntegrationDialog
      close={() => null}
      edit={() => null}
      integration={integration}
    />
  );

  // Initial state.
  expect(screen.queryByText(/required/i)).not.toBeInTheDocument();
  expect(screen.queryByTestId('scriptbox')).not.toBeInTheDocument();
  expect(screen.queryByTestId('checkbox')).not.toBeInTheDocument();
  expect(screen.getByRole('button', { name: /save/i })).toBeDisabled();
  expect(
    screen.queryByRole('button', { name: /generate command/i })
  ).not.toBeInTheDocument();

  // Changing role arn should not render generate command.
  fireEvent.change(screen.getByPlaceholderText(/arn:aws:iam:/i), {
    target: { value: 'something else' },
  });
  expect(screen.getByRole('button', { name: /save/i })).toBeEnabled();
  expect(
    screen.queryByRole('button', { name: /generate command/i })
  ).not.toBeInTheDocument();

  // Changing the s3 fields should render generate command.
  fireEvent.change(screen.getByPlaceholderText(/bucket/i), {
    target: { value: 's3-bucket-something' },
  });
  fireEvent.click(screen.getByRole('button', { name: /generate command/i }));
  expect(screen.getByRole('button', { name: /save/i })).toBeDisabled();
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

  fireEvent.click(screen.getByRole('button', { name: /save/i }));
  expect(screen.getByText(/invalid role ARN format/i)).toBeInTheDocument();

  // invalid s3 fields
  fireEvent.change(screen.getByPlaceholderText(/bucket/i), {
    target: { value: '' },
  });
  fireEvent.change(screen.getByPlaceholderText(/prefix/i), {
    target: { value: '' },
  });
  fireEvent.click(screen.getByRole('button', { name: /generate command/i }));
  expect(screen.queryAllByText(/required/i)).toHaveLength(2);
});

test('edit submit', async () => {
  const mockEditFn = jest.fn();
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

  fireEvent.click(screen.getByRole('button', { name: /generate command/i }));
  fireEvent.click(screen.getByRole('checkbox'));
  fireEvent.click(screen.getByRole('button', { name: /save/i }));

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
