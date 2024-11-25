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

import { useState } from 'react';

import { Kind, Resource } from 'teleport/services/resources';

export default function useResources<T extends Kind>(
  resources: Resource<T>[],
  templates: Templates<T>
) {
  const [state, setState] = useState({
    status: 'reading' as EditingStatus,
    item: null as Resource<T>,
  });

  const create = (kind: T) => {
    const content = templates[kind] || '';
    setState({
      status: 'creating',
      item: {
        kind,
        name: '',
        content,
        id: '',
      },
    });
  };

  const disregard = () => {
    setState({
      status: 'empty',
      item: null,
    });
  };

  const edit = (id: string) => {
    const item = resources.find(c => c.id === id);
    setState({
      status: 'editing',
      item,
    });
  };

  const remove = (id: string) => {
    const item = resources.find(c => c.id === id);
    setState({
      status: 'removing',
      item,
    });
  };

  return { ...state, create, edit, disregard, remove };
}

export type State = ReturnType<typeof useResources>;

type EditingStatus = 'creating' | 'editing' | 'removing' | 'empty';

type Templates<T extends string> = Partial<Record<T, string>>;
