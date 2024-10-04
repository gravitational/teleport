import { render, screen, userEvent } from 'design/utils/testing';
import React, { useState } from 'react';
import { StandardEditor } from './StandardEditor';
import {
  newRole,
  roleToRoleEditorModel,
  StandardEditorModel,
} from './standardmodel';
import { createTeleportContext } from 'teleport/mocks/contexts';
import TeleportContextProvider from 'teleport/TeleportContextProvider';
import { within } from '@testing-library/react';

const TestStandardEditor = () => {
  const ctx = createTeleportContext();
  const [model, setModel] = useState<StandardEditorModel>({
    roleModel: roleToRoleEditorModel(newRole()),
    isDirty: true,
  });
  return (
    <TeleportContextProvider ctx={ctx}>
      <StandardEditor
        originalRole={null}
        standardEditorModel={model}
        isProcessing={false}
        onChange={setModel}
      />
    </TeleportContextProvider>
  );
};

test('adding and removing sections', async () => {
  const user = userEvent.setup();
  const ctx = createTeleportContext();
  render(<TestStandardEditor />);
  expect(getAllSectionNames()).toEqual(['Role Metadata']);

  await user.click(
    screen.getByRole('button', { name: 'Add New Specifications' })
  );
  expect(getAllMenuItemNames()).toEqual(['Kubernetes', 'Servers']);

  await user.click(screen.getByRole('menuitem', { name: 'Servers' }));
  expect(getAllSectionNames()).toEqual(['Role Metadata', 'Servers']);

  await user.click(
    screen.getByRole('button', { name: 'Add New Specifications' })
  );
  expect(getAllMenuItemNames()).toEqual(['Kubernetes']);

  await user.click(screen.getByRole('menuitem', { name: 'Kubernetes' }));
  expect(getAllSectionNames()).toEqual([
    'Role Metadata',
    'Servers',
    'Kubernetes',
  ]);

  await user.click(
    within(getSectionByName('Servers')).getByRole('button', {
      name: 'Remove section',
    })
  );
  expect(getAllSectionNames()).toEqual(['Role Metadata', 'Kubernetes']);

  await user.click(
    within(getSectionByName('Kubernetes')).getByRole('button', {
      name: 'Remove section',
    })
  );
  expect(getAllSectionNames()).toEqual(['Role Metadata']);
});

const getAllMenuItemNames = () =>
  screen.queryAllByRole('menuitem').map(m => m.textContent);

const getAllSectionNames = () =>
  screen.queryAllByRole('heading', { level: 3 }).map(m => m.textContent);

const getSectionByName = (name: string) =>
  screen.getByRole('heading', { level: 3, name }).closest('section');
