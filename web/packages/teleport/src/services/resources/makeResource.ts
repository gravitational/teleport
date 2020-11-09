import { Resource } from './types';
import { at } from 'lodash';

export default function makeResource(json: object): Resource {
  const [id, kind, name, content] = at(json, ['id', 'kind', 'name', 'content']);
  return {
    id,
    kind,
    name,
    displayName: name,
    content,
  };
}
