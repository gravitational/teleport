import { AccessMonitoringRuleWithYaml } from 'e-teleport/services/accessmonitoringrule/types';
import { Plugin } from 'teleport/services/integrations';

export type RowBase = {
  name: string;
  types: string[];
  integration: string;
  recipients: string[];
};

export type TableRowRule = RowBase & {
  item: AccessMonitoringRuleWithYaml;
  plugin?: never;
};

export type TableRowFallback = RowBase & {
  plugin: Plugin;
  item?: never;
};

export type TableRow = TableRowRule | TableRowFallback;
