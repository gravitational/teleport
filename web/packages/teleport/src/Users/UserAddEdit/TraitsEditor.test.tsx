import React from 'react';
import { fireEvent, render, screen } from 'design/utils/testing';

import Validation from 'shared/components/Validation';

import { AllUserTraits } from 'teleport/services/user';

import { TraitsEditor, emptyTrait } from './TraitsEditor';

import { traitsToTraitsOption } from './useDialog';

import type { TraitsOption } from './TraitsEditor';

describe('Render traits correctly', () => {
  const userTraits: AllUserTraits = {
    logins: ['root', 'ubuntu'],
    db_roles: ['dbadmin', 'postgres'],
    db_names: ['postgres', 'aurora'],
  };

  test('Available traits are rendered', async () => {
    // const configuredTraits: TraitsOption[] = [];
    const setConfiguredTraits = jest.fn();

    render(
      <Validation>
        <TraitsEditor
          attempt={{ status: '' }}
          configuredTraits={traitsToTraitsOption(userTraits)}
          setConfiguredTraits={setConfiguredTraits}
        />
      </Validation>
    );

    expect(screen.getByText('User Traits')).toBeInTheDocument();
    expect(screen.getAllByTestId('trait-key')).toHaveLength(3);
    expect(screen.getAllByTestId('trait-value')).toHaveLength(3);
  });

  test('Add and remove Trait', async () => {
    const configuredTraits: TraitsOption[] = [];
    const setConfiguredTraits = jest.fn();

    const { rerender } = render(
      <Validation>
        <TraitsEditor
          attempt={{ status: '' }}
          configuredTraits={configuredTraits}
          setConfiguredTraits={setConfiguredTraits}
        />
      </Validation>
    );
    expect(screen.queryAllByTestId('trait-key')).toHaveLength(0);

    const addButtonEl = screen.getByRole('button', { name: /Add user trait/i });
    expect(addButtonEl).toBeInTheDocument();
    fireEvent.click(addButtonEl);

    expect(setConfiguredTraits).toHaveBeenLastCalledWith([emptyTrait]);

    const singleTrait = { logins: ['root', 'ubuntu'] };
    rerender(
      <Validation>
        <TraitsEditor
          attempt={{ status: '' }}
          configuredTraits={traitsToTraitsOption(singleTrait)}
          setConfiguredTraits={setConfiguredTraits}
        />
      </Validation>
    );
    fireEvent.click(screen.getByTitle('Remove Trait'));
    expect(setConfiguredTraits).toHaveBeenCalledWith([]);
  });
});
