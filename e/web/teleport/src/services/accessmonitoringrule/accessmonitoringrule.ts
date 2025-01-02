import cfg from 'e-teleport/config';
import { ResourcesResponse } from 'teleport/services/agents';
import api from 'teleport/services/api';

import {
  AccessMonitoringRuleFilter,
  AccessMonitoringRulePage,
  AccessMonitoringRuleSubject,
  AccessMonitoringRuleUpsertRequest,
  AccessMonitoringRuleWithYaml,
} from './types';

export const accessMonitoringRuleService = {
  fetchAccessMonitoringRules(
    clusterId: string,
    filter: AccessMonitoringRuleFilter
  ): Promise<AccessMonitoringRulePage> {
    return api
      .get(cfg.getAccessMonitoringRulesUrl(clusterId, filter))
      .then(resp => ({ rules: resp.rules ?? [], startKey: resp.startKey }));
  },

  fetchAccessMonitoringRulesForAccessRequests(
    clusterId: string,
    filter: AccessMonitoringRuleFilter,
    signal: AbortSignal
  ): Promise<ResourcesResponse<AccessMonitoringRuleWithYaml>> {
    return api
      .get(
        cfg.getAccessMonitoringRulesUrl(clusterId, {
          ...filter,
          subject: AccessMonitoringRuleSubject.AccessRequest,
        }),
        signal
      )
      .then(resp => {
        resp.rules = resp.rules ?? [];
        return resp;
      });
  },

  createAccessMonitoringRule(
    clusterId,
    req: AccessMonitoringRuleUpsertRequest
  ): Promise<AccessMonitoringRuleWithYaml> {
    return api.post(cfg.getAccessMonitoringRuleCreateUrl(clusterId), req);
  },

  updateAccessMonitoringRule(
    { name, clusterId }: { name: string; clusterId: string },
    req: AccessMonitoringRuleUpsertRequest
  ): Promise<AccessMonitoringRuleWithYaml> {
    return api.put(cfg.getAccessMonitoringRuleUpdateUrl(clusterId, name), req);
  },

  deleteAccessMonitoringRule({
    name,
    clusterId,
  }: {
    name: string;
    clusterId: string;
  }): Promise<void> {
    return api.delete(cfg.getAccessMonitoringRuleDeleteUrl(clusterId, name));
  },
};
