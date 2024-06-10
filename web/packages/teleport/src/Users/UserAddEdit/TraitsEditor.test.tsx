import React from 'react';
import { fireEvent, render, screen } from 'design/utils/testing';

import Validation from 'shared/components/Validation';

import { AllUserTraits } from 'teleport/services/user';

import { TraitsEditor, traitsToTraitsOption, emptyTrait } from './TraitsEditor';

import type { TraitsOption } from './TraitsEditor';

describe('Render traits correctly', () => {
  const userTraits = {
    logins: ['root', 'ubuntu'],
    db_roles: ['dbadmin', 'postgres'],
    db_names: ['postgres', 'aurora'],
  } as AllUserTraits;

  test('Avalable traits are rendered', async () => {
    let configuredTraits = [] as TraitsOption[];

    const setConfiguredTraits = jest.fn();

    const { rerender } = render(
      <TraitsEditor
        allTraits={userTraits}
        configuredTraits={configuredTraits}
        setConfiguredTraits={setConfiguredTraits}
      />
    );

    expect(screen.getByText('User Traits')).toBeInTheDocument();

    expect(setConfiguredTraits).toHaveBeenLastCalledWith(
      traitsToTraitsOption(userTraits)
    );

    rerender(
      <Validation>
        <TraitsEditor
          allTraits={userTraits}
          configuredTraits={traitsToTraitsOption(userTraits)}
          setConfiguredTraits={setConfiguredTraits}
        />
      </Validation>
    );
    expect(screen.getAllByTestId('trait-key')).toHaveLength(3);
    expect(screen.getAllByTestId('trait-value')).toHaveLength(3);
  });

  test('Add and remove Trait', async () => {
    const userTraits = {} as AllUserTraits;

    let configuredTraits = [] as TraitsOption[];

    const setConfiguredTraits = jest.fn();

    const { rerender } = render(
      <Validation>
        <TraitsEditor
          allTraits={userTraits}
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
          allTraits={singleTrait}
          configuredTraits={traitsToTraitsOption(singleTrait)}
          setConfiguredTraits={setConfiguredTraits}
        />
      </Validation>
    );
    fireEvent.click(screen.getByTitle('Remove Trait'));
    expect(setConfiguredTraits).toHaveBeenCalledWith([]);
  });
});
