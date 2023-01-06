import { AgentLabel } from 'teleport/services/agents';

import { ResourceUrlQueryParams } from './getUrlQueryParams';
import encodeUrlQueryParams from './encodeUrlQueryParams';

export default function labelClick(
  label: AgentLabel,
  params: ResourceUrlQueryParams,
  setParams: (params: ResourceUrlQueryParams) => void,
  pathname: string,
  replaceHistory: (path: string) => void
) {
  const queryParts: string[] = [];

  // Add existing query
  if (params.query) {
    queryParts.push(params.query);
  }

  // If there is an existing simple search, convert it to predicate language and add it
  if (params.search) {
    queryParts.push(`search("${params.search}")`);
  }

  const labelQuery = `labels["${label.name}"] == "${label.value}"`;
  queryParts.push(labelQuery);

  const finalQuery = queryParts.join(' && ');

  setParams({ ...params, search: '', query: finalQuery });
  replaceHistory(encodeUrlQueryParams(pathname, finalQuery, params.sort, true));
}
