import { useState } from 'shared/hooks';
import { Resource, ResourceKind } from 'e-shared/services/resources';

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
