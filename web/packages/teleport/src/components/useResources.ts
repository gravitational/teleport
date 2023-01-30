/*
Copyright 2019-2021 Gravitational, Inc.

Licensed under the Apache License, Version 2.0 (the "License");
you may not use this file except in compliance with the License.
You may obtain a copy of the License at

    http://www.apache.org/licenses/LICENSE-2.0

Unless required by applicable law or agreed to in writing, software
distributed under the License is distributed on an "AS IS" BASIS,
WITHOUT WARRANTIES OR CONDITIONS OF ANY KIND, either express or implied.
See the License for the specific language governing permissions and
limitations under the License.
*/

import { useState } from 'shared/hooks';

import { Resource, Kind } from 'teleport/services/resources';

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
