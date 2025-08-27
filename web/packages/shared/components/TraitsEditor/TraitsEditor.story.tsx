/**
 * Teleport
 * Copyright (C) 2025 Gravitational, Inc.
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

import { useState } from 'react';

import Validation from 'shared/components/Validation';

import { TraitsEditor, TraitsOption } from './TraitsEditor';

export default {
  title: 'Shared/TraitsEditor',
};

export const TraitsEditorBasic = () => {
  const [traits, setTraits] = useState<TraitsOption[]>(mockConfiguredTraits);

  return (
    <Validation>
      <TraitsEditor
        isLoading={false}
        configuredTraits={traits}
        setConfiguredTraits={setTraits}
      />
    </Validation>
  );
};

export const TraitsEditorWithToolTip = () => {
  const [traits, setTraits] = useState<TraitsOption[]>([
    {
      traitKey: { label: 'level', value: 'level' },
      traitValues: [{ label: 'L1', value: 'L1' }],
    },
    {
      traitKey: { label: 'team', value: 'team' },
      traitValues: [{ label: 'Cloud', value: 'Cloud' }],
    },
  ]);

  const tooltip = (
    <>
      If a Teleport user with the following user traits creates an Access
      Request that triggers this Access Monitoring Rule, the Access Request is
      automatically approved.
    </>
  );

  return (
    <Validation>
      <TraitsEditor
        isLoading={false}
        configuredTraits={traits}
        setConfiguredTraits={setTraits}
        tooltipContent={tooltip}
      />
    </Validation>
  );
};

export const TraitsEditorWithCustomLabels = () => {
  const [traits, setTraits] = useState<TraitsOption[]>(mockConfiguredTraits);

  return (
    <Validation>
      <TraitsEditor
        isLoading={false}
        configuredTraits={traits}
        setConfiguredTraits={setTraits}
        addActionLabel="Custom Action"
        addActionSubsequentLabel="Custom Subsequent Action"
      />
    </Validation>
  );
};

const mockConfiguredTraits = [
  {
    traitKey: { label: 'logins', value: 'logins' },
    traitValues: [{ label: 'root', value: 'root' }],
  },
];
