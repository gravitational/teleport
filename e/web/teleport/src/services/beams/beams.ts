import cfg from 'e-teleport/config';
import api from 'teleport/services/api/api';
import { withGenericUnsupportedError } from 'teleport/services/version/unsupported';

import {
  Beam,
  BeamsListParams,
  BeamsListResponse,
  CreateBeamRequest,
} from './types';
import { parseBeamResponse, parseListBeamsResponse } from './validation';

// TODO(nibrasohin) DELETE IN v20.0.0
const SUPPORTED_VERSION = '19.0.0';

const validateListResponse = (data: unknown) => {
  if (!parseListBeamsResponse(data)) {
    throw new Error('failed to parse list beams response');
  }
  return data;
};

const validateBeamResponse = (data: unknown) => {
  if (!parseBeamResponse(data)) {
    throw new Error('failed to parse beam response');
  }
  return data;
};

const handleUnsupported = (err: unknown) =>
  withGenericUnsupportedError(err, SUPPORTED_VERSION);

export const beamsService = {
  listBeams(
    params: BeamsListParams,
    clusterId: string,
    signal?: AbortSignal
  ): Promise<BeamsListResponse> {
    const { pageToken, pageSize, sortField, sortDir, users } = params;

    const qs = new URLSearchParams();
    qs.set('page_token', pageToken);
    qs.set('page_size', pageSize.toFixed());
    if (sortField) qs.set('sort_field', sortField);
    if (sortDir) qs.set('sort_dir', sortDir);
    users?.forEach(u => qs.set('user', u));

    const path = `${cfg.getBeamsUrl({ clusterId })}?${qs.toString()}`;
    return api
      .get(path, signal)
      .then(validateListResponse)
      .catch(handleUnsupported);
  },

  getBeam(
    clusterId: string,
    name: string,
    signal?: AbortSignal
  ): Promise<Beam> {
    return api
      .get(cfg.getBeamsUrl({ clusterId, name }), signal)
      .then(validateBeamResponse)
      .catch(handleUnsupported);
  },

  createBeam(clusterId: string, req: CreateBeamRequest = {}): Promise<Beam> {
    return api
      .post(cfg.getBeamsUrl({ clusterId }), req)
      .then(validateBeamResponse)
      .catch(handleUnsupported);
  },

  updateBeam(
    { clusterId, name }: { clusterId: string; name: string },
    beam: Beam
  ): Promise<Beam> {
    return api
      .put(cfg.getBeamsUrl({ clusterId, name }), beam)
      .then(validateBeamResponse)
      .catch(handleUnsupported);
  },

  deleteBeam({
    clusterId,
    name,
  }: {
    clusterId: string;
    name: string;
  }): Promise<void> {
    return api.deleteWithOptions(cfg.getBeamsUrl({ clusterId, name }));
  },
};
