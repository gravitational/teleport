import cfg from 'e-teleport/config';
import api from 'teleport/services/api/api';
import { withGenericUnsupportedError } from 'teleport/services/version/unsupported';

import {
  Beam,
  BeamsListResponse,
  CreateBeamRequest,
  BeamsListParams,
} from './types';
import { parseBeamResponse, parseListBeamsResponse } from './validation';

export const beamsService = {
  async listBeams(
    params: BeamsListParams,
    clusterId: string,
    signal?: AbortSignal
  ): Promise<BeamsListResponse> {
    const { pageToken, pageSize, sortField, sortDir, users } = params;

    const path = cfg.getBeamsUrl({ clusterId });
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
      // TODO(nibrasohin) DELETE IN v20.0.0
      withGenericUnsupportedError(err, '19.0.0');
    }
  },

  async getBeam(
    clusterId: string,
    name: string,
    signal?: AbortSignal
  ): Promise<Beam> {
    try {
      const data = await api.get(cfg.getBeamsUrl({ clusterId, name }), signal);

      if (!parseBeamResponse(data)) {
        throw new Error('failed to parse beam response');
      }

      return data;
    } catch (err) {
      // TODO(nibrasohin) DELETE IN v20.0.0
      withGenericUnsupportedError(err, '19.0.0');
    }
  },

  async createBeam(
    clusterId: string,
    req: CreateBeamRequest = {}
  ): Promise<Beam> {
    try {
      const data = await api.post(cfg.getBeamsUrl({ clusterId }), req);

      if (!parseBeamResponse(data)) {
        throw new Error('failed to parse beam response');
      }

      return data;
    } catch (err) {
      // TODO(nibrasohin) DELETE IN v20.0.0
      withGenericUnsupportedError(err, '19.0.0');
    }
  },

  async updateBeam(
    { clusterId, name }: { clusterId: string; name: string },
    beam: Beam
  ): Promise<Beam> {
    try {
      const data = await api.put(cfg.getBeamsUrl({ clusterId, name }), beam);

      if (!parseBeamResponse(data)) {
        throw new Error('failed to parse beam response');
      }

      return data;
    } catch (err) {
      // TODO(nibrasohin) DELETE IN v20.0.0
      withGenericUnsupportedError(err, '19.0.0');
    }
  },

  async deleteBeam({
    clusterId,
    name,
  }: {
    clusterId: string;
    name: string;
  }): Promise<void> {
    return api.deleteWithOptions(cfg.getBeamsUrl({ clusterId, name }));
  },
};
