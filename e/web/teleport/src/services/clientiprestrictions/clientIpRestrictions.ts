import cfg from 'e-teleport/config';
import api from 'teleport/services/api';
import { withGenericUnsupportedError } from 'teleport/services/version/unsupported';

import {
  ClientIpRestriction,
  ClientIpRestrictionMode,
  parseClientIpRestrictionResponse,
} from './validation';

// The web UI can be ahead of the proxy it is talking to, which answers a
// path-not-found 404 for an endpoint it does not have yet.
// TODO(mcbattirola): DELETE IN v20.0.0
const SUPPORTED_VERSION = '19.0.0';

const handleUnsupported = (err: unknown) =>
  withGenericUnsupportedError(err, SUPPORTED_VERSION);

export type {
  ClientIpRestriction,
  ClientIpRestrictionMode,
  ClientIpRestrictionState,
} from './validation';

/**
 * A non-empty revision makes the write a guarded update, which fails if the
 * resource changed since it was read. Empty upserts instead, which is what a
 * resource that does not exist yet needs.
 */
export type SaveClientIpRestrictionRequest = {
  cidrs: string[];
  mode?: ClientIpRestrictionMode;
  expires?: string;
  revision: string;
};

export const clientIpRestrictionsService = {
  fetchClientIpRestriction(clusterId: string): Promise<ClientIpRestriction> {
    return api
      .get(cfg.getClientIpRestrictionUrl(clusterId))
      .then(parseClientIpRestrictionResponse)
      .catch(handleUnsupported);
  },

  /** Guarded update when req.revision is non-empty, an upsert otherwise. */
  saveClientIpRestriction(
    clusterId: string,
    req: SaveClientIpRestrictionRequest
  ): Promise<ClientIpRestriction> {
    return api
      .put(cfg.getClientIpRestrictionUrl(clusterId), {
        cidrs: req.cidrs,
        mode: req.mode,
        expires: req.expires,
        revision: req.revision,
      })
      .then(parseClientIpRestrictionResponse)
      .catch(handleUnsupported);
  },

  /**
   * @deprecated Use {@link fetchClientIpRestriction}, which also returns
   * mode/expires/status/revision. Backed by the legacy CIDR-only endpoint.
   */
  async fetchClientIpRestrictions(clusterId: string): Promise<string[]> {
    const res = await api.get(cfg.getClientIpRestrictionsUrl(clusterId));
    if (!res) return [];

    return res.map(a => a.cidr);
  },

  /**
   * @deprecated Use {@link saveClientIpRestriction}. Kept for the legacy CIDR-only
   * panel; the PUT endpoint remains backward compatible.
   */
  saveClientIpRestrictions(
    clusterId: string,
    clientIpRestrictions: string[]
  ): Promise<string[]> {
    return api.put(cfg.getClientIpRestrictionsUrl(clusterId), {
      client_ip_restrictions: clientIpRestrictions.map(val => ({ cidr: val })),
    });
  },
};
