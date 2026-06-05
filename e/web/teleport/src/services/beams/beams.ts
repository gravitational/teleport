import cfg from 'e-teleport/config';
import api from 'teleport/services/api/api';
import { withGenericUnsupportedError } from 'teleport/services/version/unsupported';

import { parseListBeamsResponse } from './api';

export async function listBeams(
  variables: {
    pageToken: string;
    pageSize: number;
    users?: string[];
    sortField: string;
    sortDir: string;
  },
  signal?: AbortSignal
) {
  const { pageToken, pageSize, sortField, sortDir, users } = variables;

  const path = cfg.getBeamsUrl({ action: 'list' });
  const qs = new URLSearchParams();
  qs.set('page_token', pageToken);
  qs.set('page_size', pageSize.toFixed());
  if (sortField) {
    qs.set('sort_field', sortField);
  }
  if (sortDir) {
    qs.set('sort_dir', sortDir);
  }
  users?.forEach(users => qs.set('user', users));

  try {
    const data = await api.get(`${path}?${qs.toString()}`, signal);

    if (!parseListBeamsResponse(data)) {
      throw new Error('failed to parse list beams response');
    }

    return data;
  } catch (err) {
    // TODO(nicholasmarais1158) DELETE IN v20.0.0
    withGenericUnsupportedError(err, '19.0.0');
  }
}
