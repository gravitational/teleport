/*
Copyright 2019-2020 Gravitational, Inc.

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
import { Resource, ResourceKind } from 'teleport/services/resources';

export default function useResources(
  resources: Resource[],
  templates: Templates
) {
  const [state, setState] = useState(defaultState);

  const create = (kind: ResourceKind) => {
    const content = templates[kind] || '';
    setState({
      status: 'creating',
      item: {
        kind,
        name: '',
        content,
        id: '',
        displayName: '',
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

type EditingStatus = 'creating' | 'editing' | 'removing' | 'empty';

type Templates = Partial<Record<ResourceKind, string>>;

const defaultState = {
  status: 'reading' as EditingStatus,
  item: null as Resource,
};
