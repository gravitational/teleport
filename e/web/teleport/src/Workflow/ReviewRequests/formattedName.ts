import { ResourceId } from 'e-teleport/services/workflow';

export function formattedName(id: ResourceId) {
  return id.subResourceName ? `${id.name}/${id.subResourceName}` : id.name;
}
