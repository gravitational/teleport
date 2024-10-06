import { render, screen, userEvent } from 'design/utils/testing';
import React, { useState } from 'react';

import { within } from '@testing-library/react';
import Validation from 'shared/components/Validation';
import selectEvent from 'react-select-event';

import TeleportContextProvider from 'teleport/TeleportContextProvider';
import { createTeleportContext } from 'teleport/mocks/contexts';

import {
  newRole,
  roleToRoleEditorModel,
  ServerAccessSpec,
  StandardEditorModel,
} from './standardmodel';
import { ServerAccessSpecSection, StandardEditor } from './StandardEditor';

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
  // There's no better way to do it, unfortunately.
  // eslint-disable-next-line testing-library/no-node-access
  screen.getByRole('heading', { level: 3, name }).closest('section');

const TestServerAccessSpecsSection = ({
  onChange,
}: {
  onChange(spec: ServerAccessSpec): void;
}) => {
  const [model, setModel] = useState<ServerAccessSpec>({
    kind: 'node',
    labels: [],
    logins: [],
  });
  return (
    <Validation>
      <ServerAccessSpecSection
        value={model}
        isProcessing={false}
        onChange={spec => {
          setModel(spec);
          onChange(spec);
        }}
      />
    </Validation>
  );
};

test('editing server access specs', async () => {
  const user = userEvent.setup();
  const onChange = jest.fn();
  render(<TestServerAccessSpecsSection onChange={onChange} />);
  await user.click(screen.getByRole('button', { name: 'Add a Label' }));
  await user.type(screen.getByPlaceholderText('label key'), 'some-key');
  await user.type(screen.getByPlaceholderText('label value'), 'some-value');
  await selectEvent.create(screen.getByLabelText('Logins'), 'root', {
    createOptionText: 'Login: root',
  });
  await selectEvent.create(screen.getByLabelText('Logins'), 'some-user', {
    createOptionText: 'Login: some-user',
  });

  expect(onChange).toHaveBeenLastCalledWith({
    kind: 'node',
    labels: [{ name: 'some-key', value: 'some-value' }],
    logins: [newOptionFor('root'), newOptionFor('some-user')],
  } as ServerAccessSpec);
});

const newOptionFor = (value: string) => ({
  label: value,
  value,
  // This one is silently added by React Select to all newly created options.
  // Surprise!
  __isNew__: true,
});
