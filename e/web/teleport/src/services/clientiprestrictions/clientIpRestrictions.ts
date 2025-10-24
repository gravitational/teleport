import cfg from 'e-teleport/config';
import api from 'teleport/services/api';

export const clientIpRestrictionsService = {
  async fetchClientIpRestrictions(clusterId: string): Promise<string[]> {
    const res = await api.get(cfg.getClientIpRestrictionsUrl(clusterId));
    if (!res) return [];

    return res.map(a => a.cidr);
  },

  saveClientIpRestrictions(
    clusterId: string,
    clientIpRestrictions: string[]
  ): Promise<string[]> {
    return api.put(cfg.getClientIpRestrictionsUrl(clusterId), {
      client_ip_restrictions: clientIpRestrictions.map(val => ({ cidr: val })),
    });
  },
};
