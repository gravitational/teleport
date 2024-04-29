import { Resource } from 'e-teleport/services/accessRequests';

export function formattedName(resource: Resource) {
  const id = resource.id;
  if (id.subResourceName) {
    return `${id.name}/${id.subResourceName}`;
  }
  return id.name;
}
