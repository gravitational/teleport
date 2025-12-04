import { accessMonitoringRuleService } from 'e-teleport/services/accessmonitoringrule';
import {
  AccessMonitoringRuleFilter,
  AccessMonitoringRuleSubject,
  AccessMonitoringRuleWithYaml,
} from 'e-teleport/services/accessmonitoringrule/types';
import { yamlService } from 'teleport/services/yaml';
import { YamlSupportedResourceKind } from 'teleport/services/yaml/types';
import TeleportContext from 'teleport/teleportContext';
import useStickyClusterId from 'teleport/useStickyClusterId';

export function useRules(ctx: TeleportContext) {
  const rulesAcl = ctx.storeUser.getAccessMonitoringRuleAccess();
  const { clusterId } = useStickyClusterId();

  async function create(rule: Partial<AccessMonitoringRuleWithYaml>) {
    return accessMonitoringRuleService.createAccessMonitoringRule(clusterId, {
      yaml: await toYaml(rule),
    });
  }

  async function update(
    name: string,
    rule: Partial<AccessMonitoringRuleWithYaml>
  ) {
    return accessMonitoringRuleService.updateAccessMonitoringRule(
      { name, clusterId },
      { yaml: await toYaml(rule) }
    );
  }

  function remove(name: string) {
    return accessMonitoringRuleService.deleteAccessMonitoringRule({
      name,
      clusterId,
    });
  }

  function fetch(params?: AccessMonitoringRuleFilter) {
    return accessMonitoringRuleService.fetchAccessMonitoringRules(clusterId, {
      ...params,
      subject: AccessMonitoringRuleSubject.AccessRequest,
    });
  }

  return {
    fetch,
    create,
    update,
    remove,
    rulesAcl,
  };
}

async function toYaml(
  rule: Partial<AccessMonitoringRuleWithYaml>
): Promise<string> {
  return (
    rule.yaml ||
    (await yamlService.stringify(
      YamlSupportedResourceKind.AccessMonitoringRule,
      {
        resource: rule.object,
      }
    ))
  );
}

export type RulesState = ReturnType<typeof useRules>;
