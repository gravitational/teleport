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

import { render, screen, userEvent } from 'design/utils/testing';
import { useState } from 'react';
import { within } from '@testing-library/react';
import Validation from 'shared/components/Validation';
import TeleportContextProvider from 'teleport/TeleportContextProvider';
import { createTeleportContext } from 'teleport/mocks/contexts';

import {
  newRole,
  roleToRoleEditorModel,
  StandardEditorModel,
} from './standardmodel';
import { StandardEditor, StandardEditorProps } from './StandardEditor';

const TestStandardEditor = (props: Partial<StandardEditorProps>) => {
  const ctx = createTeleportContext();
  const [model, setModel] = useState<StandardEditorModel>({
    roleModel: roleToRoleEditorModel(newRole()),
    isDirty: true,
  });
  return (
    <TeleportContextProvider ctx={ctx}>
      <Validation>
        <StandardEditor
          originalRole={null}
          standardEditorModel={model}
          isProcessing={false}
          onChange={setModel}
          {...props}
        />
      </Validation>
    </TeleportContextProvider>
  );
};

test('adding and removing sections', async () => {
  const user = userEvent.setup();
  render(<TestStandardEditor />);
  expect(getAllSectionNames()).toEqual(['Role Metadata']);
  await user.click(screen.getByRole('tab', { name: 'Resources' }));
  expect(getAllSectionNames()).toEqual([]);

  await user.click(
    screen.getByRole('button', { name: 'Add New Resource Access' })
  );
  expect(getAllMenuItemNames()).toEqual([
    'Kubernetes',
    'Servers',
    'Applications',
    'Databases',
    'Windows Desktops',
  ]);

  await user.click(screen.getByRole('menuitem', { name: 'Servers' }));
  expect(getAllSectionNames()).toEqual(['Servers']);

  await user.click(
    screen.getByRole('button', { name: 'Add New Resource Access' })
  );
  expect(getAllMenuItemNames()).toEqual([
    'Kubernetes',
    'Applications',
    'Databases',
    'Windows Desktops',
  ]);

  await user.click(screen.getByRole('menuitem', { name: 'Kubernetes' }));
  expect(getAllSectionNames()).toEqual(['Servers', 'Kubernetes']);

  await user.click(
    within(getSectionByName('Servers')).getByRole('button', {
      name: 'Remove section',
    })
  );
  expect(getAllSectionNames()).toEqual(['Kubernetes']);

  await user.click(
    within(getSectionByName('Kubernetes')).getByRole('button', {
      name: 'Remove section',
    })
  );
  expect(getAllSectionNames()).toEqual([]);
});

test('collapsed sections still apply validation', async () => {
  const user = userEvent.setup();
  const onSave = jest.fn();
  render(<TestStandardEditor onSave={onSave} />);
  // Intentionally cause a validation error.
  await user.clear(screen.getByLabelText('Role Name'));
  // Collapse the section.
  await user.click(screen.getByRole('heading', { name: 'Role Metadata' }));
  await user.click(screen.getByRole('button', { name: 'Create Role' }));
  expect(onSave).not.toHaveBeenCalled();

  // Expand the section, make it valid.
  await user.click(screen.getByRole('heading', { name: 'Role Metadata' }));
  await user.type(screen.getByLabelText('Role Name'), 'foo');
  await user.click(screen.getByRole('button', { name: 'Create Role' }));
  expect(onSave).toHaveBeenCalled();
});

test('invisible tabs still apply validation', async () => {
  const user = userEvent.setup();
  const onSave = jest.fn();
  render(<TestStandardEditor onSave={onSave} />);
  // Intentionally cause a validation error.
  await user.clear(screen.getByLabelText('Role Name'));
  // Switch to a different tab.
  await user.click(screen.getByRole('tab', { name: 'Resources' }));
  await user.click(screen.getByRole('button', { name: 'Create Role' }));
  expect(onSave).not.toHaveBeenCalled();

  // Switch back, make it valid.
  await user.click(screen.getByRole('tab', { name: 'Invalid data Overview' }));
  await user.type(screen.getByLabelText('Role Name'), 'foo');
  await user.click(screen.getByRole('button', { name: 'Create Role' }));
  expect(onSave).toHaveBeenCalled();
});

const getAllMenuItemNames = () =>
  screen.queryAllByRole('menuitem').map(m => m.textContent);

const getAllSectionNames = () =>
  screen.queryAllByRole('heading', { level: 3 }).map(m => m.textContent);

const getSectionByName = (name: string) =>
  // There's no better way to do it, unfortunately.
  // eslint-disable-next-line testing-library/no-node-access
  screen.getByRole('heading', { level: 3, name }).closest('details');
