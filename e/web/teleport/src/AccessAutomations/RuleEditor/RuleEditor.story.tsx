import { AccessMonitoringRuleType } from 'e-teleport/services/accessmonitoringrule/types';
import { TeleportProviderBasic } from 'teleport/mocks/providers';
import { Plugin } from 'teleport/services/integrations';

import { RuleEditor } from './RuleEditor';

export default {
  title: 'TeleportE/AccessAutomations/RuleEditor',
};

export const Notification = () => {
  return <Component editor={AccessMonitoringRuleType.Notification} />;
};

export const Review = () => {
  return <Component editor={AccessMonitoringRuleType.Review} />;
};

const Component = ({
  editor = AccessMonitoringRuleType.Notification,
}: {
  editor?: AccessMonitoringRuleType;
}) => {
  return (
    <TeleportProviderBasic>
      <RuleEditor {...sample} editor={editor} />
    </TeleportProviderBasic>
  );
};

const plugins: Plugin[] = [
  {
    resourceType: 'plugin',
    kind: 'slack',
    name: 'slack-plugin',
    statusCode: 1,
    spec: { fallbackChannel: 'some-fallback-channel' },
  },
];

const sample = {
  onCancel: () => null,
  onEdit: () => null,
  onDelete: () => null,
  onSave: () => null,
  plugins: plugins,
  editor: AccessMonitoringRuleType.Notification,
};
