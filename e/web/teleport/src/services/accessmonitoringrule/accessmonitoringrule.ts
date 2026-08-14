import cfg from 'e-teleport/config';
import { ResourcesResponse } from 'teleport/services/agents';
import api from 'teleport/services/api';

import {
  AccessMonitoringRuleFilter,
  AccessMonitoringRuleUpsertRequest,
  AccessMonitoringRuleWithYaml,
} from './types';

export const accessMonitoringRuleService = {
  fetchAccessMonitoringRules(
    clusterId: string,
    filter: AccessMonitoringRuleFilter,
    signal?: AbortSignal
  ): Promise<ResourcesResponse<AccessMonitoringRuleWithYaml>> {
    return api
      .get(cfg.getAccessMonitoringRulesUrl(clusterId, filter), signal)
      .then(resp => ({ agents: resp.rules ?? [], startKey: resp.startKey }));
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

  fetchAccessMonitoringRuleTerraform(
    { clusterId, name }: { clusterId: string; name: string },
    signal?: AbortSignal
  ): Promise<string> {
    return api
      .get(cfg.getAccessMonitoringRuleTerraformUrl(clusterId, name), signal)
      .then(resp => resp.terraform);
  },
};
