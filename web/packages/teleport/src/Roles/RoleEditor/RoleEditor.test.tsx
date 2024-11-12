/**
 * Teleport
 * Copyright (C) 2024 Gravitational, Inc.
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

import React from 'react';
import { render, screen, userEvent } from 'design/utils/testing';
import { within } from '@testing-library/react';
import { UserEvent } from '@testing-library/user-event';

import { Role } from 'teleport/services/resources';
import { createTeleportContext } from 'teleport/mocks/contexts';
import TeleportContextProvider from 'teleport/TeleportContextProvider';
import { yamlService } from 'teleport/services/yaml';
import {
  YamlStringifyRequest,
  YamlSupportedResourceKind,
} from 'teleport/services/yaml/types';
import { CaptureEvent, userEventService } from 'teleport/services/userEvent';

import { RoleEditor, RoleEditorProps } from './RoleEditor';
import { defaultOptions, withDefaults } from './withDefaults';

// The Ace editor is very difficult to deal with in tests, especially that for
// handling its state, we are using input event, which is asynchronous. Thus,
// for testing, we just mock it out with a simple text area.
jest.mock('shared/components/TextEditor', () => ({
  __esModule: true,
  default: ({
    data: [{ content }],
    onChange,
  }: {
    data: { content: string }[];
    onChange(s: string): void;
  }) => <textarea value={content} onChange={e => onChange(e.target.value)} />,
}));

let user: UserEvent;

beforeEach(() => {
  user = userEvent.setup();
  jest.spyOn(yamlService, 'parse').mockImplementation(async (kind, req) => {
    if (kind != YamlSupportedResourceKind.Role) {
      throw new Error(`Wrong kind: ${kind}`);
    }
    return withDefaults(fromFauxYaml(req.yaml));
  });
  jest
    .spyOn(yamlService, 'stringify')
    .mockImplementation(async (kind, req: YamlStringifyRequest<Role>) => {
      if (kind != YamlSupportedResourceKind.Role) {
        throw new Error(`Wrong kind: ${kind}`);
      }
      return toFauxYaml(withDefaults(req.resource));
    });
  jest.spyOn(userEventService, 'captureUserEvent').mockImplementation(() => {});
});

afterEach(() => {
  jest.restoreAllMocks();
});

test('rendering and switching tabs for new role', async () => {
  render(<TestRoleEditor />);
  expect(screen.getByRole('tab', { name: 'Standard' })).toHaveAttribute(
    'aria-selected',
    'true'
  );
  expect(
    screen.queryByRole('button', { name: /Reset to Standard Settings/i })
  ).not.toBeInTheDocument();
  expect(screen.getByLabelText('Role Name')).toHaveValue('new_role_name');
  expect(screen.getByLabelText('Description')).toHaveValue('');
  expect(screen.getByRole('button', { name: 'Create Role' })).toBeEnabled();

  await user.click(screen.getByRole('tab', { name: 'YAML' }));
  expect(fromFauxYaml(await getTextEditorContents())).toEqual(
    withDefaults({
      kind: 'role',
      metadata: {
        name: 'new_role_name',
      },
      spec: {
        allow: {},
        deny: {},
        options: {},
      },
      version: 'v7',
    })
  );
  expect(screen.getByRole('button', { name: 'Create Role' })).toBeEnabled();

  await user.click(screen.getByRole('tab', { name: 'Standard' }));
  await screen.findByLabelText('Role Name');
  expect(
    screen.queryByRole('button', { name: /Reset to Standard Settings/i })
  ).not.toBeInTheDocument();
  expect(screen.getByRole('button', { name: 'Create Role' })).toBeEnabled();
});

test('rendering and switching tabs for a non-standard role', async () => {
  const originalRole = withDefaults({
    metadata: {
      name: 'some-role',
      revision: 'aa27b7e2-080f-4aba-9f93-c7d168505798',
    },
    spec: {
      deny: { node_labels: { foo: ['bar'] } },
    },
  });
  const originalYaml = toFauxYaml(originalRole);
  render(
    <TestRoleEditor
      originalRole={{ object: originalRole, yaml: originalYaml }}
    />
  );
  expect(screen.getByRole('tab', { name: 'YAML' })).toHaveAttribute(
    'aria-selected',
    'true'
  );
  expect(fromFauxYaml(await getTextEditorContents())).toEqual(originalRole);
  expect(screen.getByRole('button', { name: 'Update Role' })).toBeDisabled();

  await user.click(screen.getByRole('tab', { name: 'Standard' }));
  expect(
    screen.getByRole('button', { name: 'Reset to Standard Settings' })
  ).toBeVisible();
  expect(screen.getByLabelText('Role Name')).toHaveValue('some-role');
  expect(screen.getByLabelText('Description')).toHaveValue('');
  expect(screen.getByRole('button', { name: 'Update Role' })).toBeDisabled();

  await user.click(screen.getByRole('tab', { name: 'YAML' }));
  expect(fromFauxYaml(await getTextEditorContents())).toEqual(originalRole);
  expect(screen.getByRole('button', { name: 'Update Role' })).toBeDisabled();

  // Switch once again, reset to standard
  await user.click(screen.getByRole('tab', { name: 'Standard' }));
  expect(screen.getByRole('button', { name: 'Update Role' })).toBeDisabled();
  await user.click(
    screen.getByRole('button', { name: 'Reset to Standard Settings' })
  );
  expect(screen.getByRole('button', { name: 'Update Role' })).toBeEnabled();
  await user.type(screen.getByLabelText('Description'), 'some description');

  await user.click(screen.getByRole('tab', { name: 'YAML' }));
  const editorContents = fromFauxYaml(await getTextEditorContents());
  expect(editorContents.metadata.description).toBe('some description');
  expect(editorContents.spec.deny).toEqual({});
  expect(screen.getByRole('button', { name: 'Update Role' })).toBeEnabled();
});

test('canceling standard editor', async () => {
  const onCancel = jest.fn();
  render(<TestRoleEditor onCancel={onCancel} />);
  await user.click(screen.getByRole('button', { name: 'Cancel' }));
  expect(onCancel).toHaveBeenCalled();
  expect(userEventService.captureUserEvent).toHaveBeenCalledWith({
    event: CaptureEvent.CreateNewRoleCancelClickEvent,
  });
});

test('canceling yaml editor', async () => {
  const onCancel = jest.fn();
  render(<TestRoleEditor onCancel={onCancel} />);
  await user.click(screen.getByRole('tab', { name: 'YAML' }));
  await user.click(screen.getByRole('button', { name: 'Cancel' }));
  expect(onCancel).toHaveBeenCalled();
  expect(userEventService.captureUserEvent).toHaveBeenCalledWith({
    event: CaptureEvent.CreateNewRoleCancelClickEvent,
  });
});

test('saving a new role', async () => {
  const onSave = jest.fn();
  render(<TestRoleEditor onSave={onSave} />);
  expect(screen.getByRole('button', { name: 'Create Role' })).toBeEnabled();

  await user.clear(screen.getByLabelText('Role Name'));
  await user.type(screen.getByLabelText('Role Name'), 'great-old-one');
  await user.clear(screen.getByLabelText('Description'));
  await user.type(
    screen.getByLabelText('Description'),
    'That is not dead which can eternal lie.'
  );
  await user.click(screen.getByRole('button', { name: 'Create Role' }));

  expect(onSave).toHaveBeenCalledWith({
    object: {
      kind: 'role',
      metadata: {
        name: 'great-old-one',
        description: 'That is not dead which can eternal lie.',
      },
      spec: {
        allow: {},
        deny: {},
        options: defaultOptions(),
      },
      version: 'v7',
    },
  });
  expect(userEventService.captureUserEvent).toHaveBeenCalledWith({
    event: CaptureEvent.CreateNewRoleSaveClickEvent,
  });
});

test('saving a new role after editing as YAML', async () => {
  const onSave = jest.fn();
  render(<TestRoleEditor onSave={onSave} />);
  expect(screen.getByRole('button', { name: 'Create Role' })).toBeEnabled();

  await user.click(screen.getByRole('tab', { name: 'YAML' }));
  await user.clear(await findTextEditor());
  await user.type(await findTextEditor(), '{{"foo":"bar"}');
  await user.click(screen.getByRole('button', { name: 'Create Role' }));

  expect(onSave).toHaveBeenCalledWith({
    yaml: '{"foo":"bar"}',
  });
  expect(userEventService.captureUserEvent).toHaveBeenCalledWith({
    event: CaptureEvent.CreateNewRoleSaveClickEvent,
  });
});

test('error while saving', async () => {
  const onSave = jest.fn().mockRejectedValue(new Error('oh noes'));
  render(<TestRoleEditor onSave={onSave} />);
  await user.click(screen.getByRole('button', { name: 'Create Role' }));
  expect(screen.getByText('oh noes')).toBeVisible();
});

test('error while yamlifying', async () => {
  jest
    .spyOn(yamlService, 'stringify')
    .mockRejectedValue(new Error('me no speak yaml'));
  render(<TestRoleEditor />);
  await user.click(screen.getByRole('tab', { name: 'YAML' }));
  expect(screen.getByText('me no speak yaml')).toBeVisible();
});

test('error while parsing', async () => {
  jest
    .spyOn(yamlService, 'parse')
    .mockRejectedValue(new Error('me no speak yaml'));
  render(<TestRoleEditor />);
  await user.click(screen.getByRole('tab', { name: 'YAML' }));
  await user.click(screen.getByRole('tab', { name: 'Standard' }));
  expect(screen.getByText('me no speak yaml')).toBeVisible();
});

// Here's a trick: since we can't parse YAML back and forth, we use a
// "pretended YAML", which is actually JSON. It's not so far-fetched, as JSON
// is a subset of YAML.

/** Pretends to parse YAML, but in reality, parses JSON. */
const fromFauxYaml = (s: string): Role => JSON.parse(s);

/** Pretends to stringify to YAML, but in reality, stringifies to JSON. */
const toFauxYaml = (r: Role): string => JSON.stringify(r);

const TestRoleEditor = (props: RoleEditorProps) => {
  const ctx = createTeleportContext();
  return (
    <TeleportContextProvider ctx={ctx}>
      <RoleEditor {...props} />
    </TeleportContextProvider>
  );
};

const findTextEditor = async () =>
  within(await screen.findByTestId('text-editor-container')).getByRole(
    'textbox'
  );

/**
 * Retrieves Ace text editor contents. We can't just trust the textarea
 * contents, this is unreliable. We really have to use Ace editor API to do it.
 */
const getTextEditorContents = async () => (await findTextEditor()).textContent;
