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

import { within } from '@testing-library/react';
import { UserEvent } from '@testing-library/user-event';

import { render, screen, userEvent } from 'design/utils/testing';

import cfg from 'teleport/config';
import { createTeleportContext } from 'teleport/mocks/contexts';
import { Role } from 'teleport/services/resources';
import { storageService } from 'teleport/services/storageService';
import { CaptureEvent, userEventService } from 'teleport/services/userEvent';
import { yamlService } from 'teleport/services/yaml';
import {
  YamlStringifyRequest,
  YamlSupportedResourceKind,
} from 'teleport/services/yaml/types';
import TeleportContextProvider from 'teleport/TeleportContextProvider';

import { RoleEditor, RoleEditorProps } from './RoleEditor';
import * as StandardEditorModule from './StandardEditor/StandardEditor';
import { defaultRoleVersion } from './StandardEditor/standardmodel';
import * as StandardModelModule from './StandardEditor/standardmodel';
import { defaultOptions, withDefaults } from './StandardEditor/withDefaults';

const defaultIsPolicyEnabled = cfg.isPolicyEnabled;

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
  cfg.isPolicyEnabled = defaultIsPolicyEnabled;
});

test('rendering and switching tabs for new role', async () => {
  render(<TestRoleEditor />);
  expect(getStandardEditorTab()).toHaveAttribute('aria-selected', 'true');
  expect(
    screen.queryByRole('button', { name: /Reset to Standard Settings/i })
  ).not.toBeInTheDocument();
  expect(screen.getByLabelText('Role Name *')).toHaveValue('new_role_name');
  expect(screen.getByLabelText('Description')).toHaveValue('');
  expect(screen.getByRole('button', { name: 'Create Role' })).toBeEnabled();

  await user.click(getYamlEditorTab());
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
      version: defaultRoleVersion,
    })
  );
  expect(screen.getByRole('button', { name: 'Create Role' })).toBeEnabled();

  await user.click(getStandardEditorTab());
  await screen.findByLabelText('Role Name *');
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
  expect(getYamlEditorTab()).toHaveAttribute('aria-selected', 'true');
  expect(fromFauxYaml(await getTextEditorContents())).toEqual(originalRole);
  expect(screen.getByRole('button', { name: 'Save Changes' })).toBeDisabled();

  await user.click(getStandardEditorTab());
  expect(screen.getByText(/This role is too complex/)).toBeVisible();
  expect(screen.getByLabelText('Role Name *')).toHaveValue('some-role');
  expect(screen.getByLabelText('Description')).toHaveValue('');
  expect(screen.getByRole('button', { name: 'Save Changes' })).toBeDisabled();

  await user.click(getYamlEditorTab());
  expect(fromFauxYaml(await getTextEditorContents())).toEqual(originalRole);
  expect(screen.getByRole('button', { name: 'Save Changes' })).toBeDisabled();
});

it('calls onRoleUpdate on each modification in the standard editor', async () => {
  cfg.isPolicyEnabled = true;
  const onRoleUpdate = jest.fn();
  render(<TestRoleEditor onRoleUpdate={onRoleUpdate} />);
  expect(onRoleUpdate).toHaveBeenLastCalledWith(
    withDefaults({ metadata: { name: 'new_role_name' } })
  );
  await user.type(screen.getByLabelText('Description'), 'some-description');
  expect(onRoleUpdate).toHaveBeenLastCalledWith(
    withDefaults({
      metadata: { name: 'new_role_name', description: 'some-description' },
    })
  );
});

test('switching tabs triggers validation', async () => {
  // Triggering validation is necessary, because server-side yamlification
  // sometimes will reject the data anyway.
  render(<TestRoleEditor />);
  await user.clear(screen.getByLabelText('Role Name *'));
  expect(getStandardEditorTab()).toHaveAttribute('aria-selected', 'true');
  await user.click(getYamlEditorTab());
  expect(screen.getByLabelText('Role Name *')).toHaveAccessibleDescription(
    'Role name is required'
  );
  // Expect to still be on the standard tab.
  expect(getStandardEditorTab()).toHaveAttribute('aria-selected', 'true');
});

test('switching tabs ignores standard model validation for a non-standard role', async () => {
  // The purpose of this test is to rule out a case where we start with a
  // non-standard role that even after resetting would cause a validation
  // error, then go to the standard editor only to be stuck there by the
  // validation requirement, while the editor UI is itself blocked.
  const originalRole = withDefaults({
    // This will trigger a validation error. Note that empty metadata is a very
    // blunt tool here, but that's certainly one case where we can guarantee
    // it's gonna cause validation errors. The real-world scenarios would be
    // much more subtle, but also more likely to get fixed in future, rendering
    // this test case futile.
    metadata: {},
    spec: {},
    unsupportedField: true, // This will cause disabling the standard editor.
  } as any as Role);
  render(
    <TestRoleEditor
      originalRole={{ object: originalRole, yaml: toFauxYaml(originalRole) }}
    />
  );
  expect(getYamlEditorTab()).toHaveAttribute('aria-selected', 'true');
  await user.click(getStandardEditorTab());
  expect(screen.getByText(/This role is too complex/)).toBeVisible();
  await user.click(getYamlEditorTab());
  // Proceed, even though our validation would consider the data invalid.
  expect(getYamlEditorTab()).toHaveAttribute('aria-selected', 'true');
});

test('no double conversions when clicking already active tabs', async () => {
  render(<TestRoleEditor />);
  await user.click(getYamlEditorTab());
  await user.click(getStandardEditorTab());
  await user.type(screen.getByLabelText('Role Name *'), '_2');
  await user.click(getStandardEditorTab());
  expect(screen.getByLabelText('Role Name *')).toHaveValue('new_role_name_2');

  await user.click(getYamlEditorTab());
  await user.clear(await findTextEditor());
  await user.type(
    await findTextEditor(),
    // Note: this is actually correct JSON syntax; the testing library uses
    // braces for special keys, so we need to use double opening braces.
    '{{"kind":"role", metadata:{{"name":"new_role_name_3"}}'
  );
  await user.click(getYamlEditorTab());
  expect(await getTextEditorContents()).toBe(
    '{"kind":"role", metadata:{"name":"new_role_name_3"}}'
  );
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
  await user.click(getYamlEditorTab());
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

  await user.clear(screen.getByLabelText('Role Name *'));
  await user.type(screen.getByLabelText('Role Name *'), 'great-old-one');
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

describe('saving a new role after editing as YAML', () => {
  test('with Policy disabled', async () => {
    const onSave = jest.fn();
    render(<TestRoleEditor onSave={onSave} />);
    expect(screen.getByRole('button', { name: 'Create Role' })).toBeEnabled();

    await user.click(getYamlEditorTab());
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

  test('with Policy enabled', async () => {
    cfg.isPolicyEnabled = true;
    jest
      .spyOn(storageService, 'getAccessGraphRoleTesterEnabled')
      .mockReturnValue(true);

    const onRoleUpdate = jest.fn();
    const onSave = jest.fn();
    render(<TestRoleEditor onRoleUpdate={onRoleUpdate} onSave={onSave} />);
    expect(screen.getByRole('button', { name: 'Create Role' })).toBeEnabled();

    await user.click(getYamlEditorTab());
    await user.clear(await findTextEditor());

    onRoleUpdate.mockReset();
    await user.type(
      await findTextEditor(),
      '{{"metadata":{{"description":"foo"}}'
    );
    expect(onRoleUpdate).not.toHaveBeenCalled();
    await user.click(screen.getByRole('button', { name: 'Preview' }));
    expect(onRoleUpdate).toHaveBeenCalledTimes(1);
    expect(onRoleUpdate).toHaveBeenCalledWith(
      withDefaults({ metadata: { description: 'foo' } })
    );
    await user.click(screen.getByRole('button', { name: 'Create Role' }));

    expect(onSave).toHaveBeenCalledWith({
      yaml: '{"metadata":{"description":"foo"}}',
    });
    expect(userEventService.captureUserEvent).toHaveBeenCalledWith({
      event: CaptureEvent.CreateNewRoleSaveClickEvent,
    });
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
  await user.click(getYamlEditorTab());
  expect(screen.getByText(/me no speak yaml/)).toBeVisible();
});

test('error while parsing', async () => {
  jest
    .spyOn(yamlService, 'parse')
    .mockRejectedValue(new Error('me no speak yaml'));
  render(<TestRoleEditor />);
  await user.click(getYamlEditorTab());
  await user.click(getStandardEditorTab());
  expect(
    screen.getByText('Unable to load role into the standard editor')
  ).toBeVisible();
  expect(screen.getByText(/me no speak yaml/)).toBeVisible();
});

test('YAML editor is usable even if the standard one throws', async () => {
  // Mock the standard editor to force it to throw an error.
  jest.spyOn(StandardEditorModule, 'StandardEditor').mockImplementation(() => {
    throw new Error('oh noes, it crashed');
  });
  // Ignore the error being reported on the console.
  jest.spyOn(console, 'error').mockImplementation();

  const onSave = jest.fn();
  render(<TestRoleEditor onSave={onSave} />);
  expect(getStandardEditorTab()).toHaveAttribute('aria-selected', 'true');
  expect(screen.getByText('oh noes, it crashed')).toBeVisible();

  // Expect to still be able to use to the YAML editor.
  await user.click(getYamlEditorTab());
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
      version: defaultRoleVersion,
    })
  );
  await user.clear(await findTextEditor());
  await user.type(await findTextEditor(), '{{"modified":1}');
  await user.click(screen.getByRole('button', { name: 'Create Role' }));

  expect(onSave).toHaveBeenCalledWith({
    yaml: '{"modified":1}',
  });
});

it('YAML editor usable even if the initial conversion throws', async () => {
  // Mock the role converter to force it to throw an error.
  jest
    .spyOn(StandardModelModule, 'roleToRoleEditorModel')
    .mockImplementation(() => {
      throw new Error('oh noes, it crashed');
    });
  // Ignore the error being reported on the console.
  jest.spyOn(console, 'error').mockImplementation();

  const originalRole = withDefaults({
    metadata: {
      name: 'some-role',
      revision: 'aa27b7e2-080f-4aba-9f93-c7d168505798',
    },
    spec: {
      allow: { node_labels: { foo: ['bar'] } },
    },
  });
  const originalYaml = toFauxYaml(originalRole);
  const onSave = jest.fn();
  render(
    <TestRoleEditor
      originalRole={{ object: originalRole, yaml: originalYaml }}
      onSave={onSave}
    />
  );
  expect(getYamlEditorTab()).toHaveAttribute('aria-selected', 'true');

  expect(fromFauxYaml(await getTextEditorContents())).toEqual(originalRole);
  await user.clear(await findTextEditor());
  await user.type(await findTextEditor(), '{{"modified":1}');
  await user.click(screen.getByRole('button', { name: 'Save Changes' }));

  expect(onSave).toHaveBeenCalledWith({
    yaml: '{"modified":1}',
  });

  expect(console.error).toHaveBeenCalledTimes(1);
  expect(console.error).toHaveBeenCalledWith(
    expect.any(String),
    expect.any(String),
    'Could not convert Role to a standard model',
    expect.objectContaining({
      message: expect.stringMatching('oh noes, it crashed'),
    })
  );
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

const getStandardEditorTab = () =>
  screen.getByRole('tab', { name: 'Switch to standard editor' });

const getYamlEditorTab = () =>
  screen.getByRole('tab', { name: 'Switch to YAML editor' });

const findTextEditor = async () =>
  within(await screen.findByTestId('text-editor-container')).getByRole(
    'textbox'
  );

/**
 * Retrieves Ace text editor contents. We can't just trust the textarea
 * contents, this is unreliable. We really have to use Ace editor API to do it.
 */
const getTextEditorContents = async () => (await findTextEditor()).textContent;
