import { JSX, useEffect, useState } from 'react';

import { Alert, Box, Flex, Text } from 'design';
import { HoverTooltip } from 'design/Tooltip';
import { MenuButton, MenuItem } from 'shared/components/MenuAction';
import { MissingPermissionsTooltip } from 'shared/components/MissingPermissionsTooltip';
import {
  InfoGuideButton,
  InfoParagraph,
  ReferenceLinks,
} from 'shared/components/SlidingSidePanel/InfoGuide';
import useAttempt, { Attempt } from 'shared/hooks/useAttemptNext';

import {
  AccessMonitoringRuleType,
  AccessMonitoringRuleWithYaml,
} from 'e-teleport/services/accessmonitoringrule/types';
import { pluginsService } from 'e-teleport/services/plugins';
import { useServerSidePagination } from 'teleport/components/hooks';
import {
  FeatureBox,
  FeatureHeader,
  FeatureHeaderTitle,
} from 'teleport/components/Layout';
import { Plugin } from 'teleport/services/integrations';
import useTeleport from 'teleport/useTeleport';

import { RuleEditor } from './RuleEditor/RuleEditor';
import { RuleList } from './RuleList/RuleList';
import { TerraformDialog } from './TerraformDialog/TerraformDialog';
import { useRules } from './useRules';

export function AccessAutomations() {
  const ctx = useTeleport();
  const { fetch, create, update, remove, rulesAcl } = useRules(ctx);
  const pluginAccess = ctx.storeUser.getPluginsAccess();
  const hasPluginAccess = pluginAccess.read;
  const missingPermissions = [
    { hasAccess: rulesAcl.create, label: 'access_monitoring_rule.create' },
    { hasAccess: hasPluginAccess, label: 'plugin.read' },
  ]
    .filter(perm => !perm.hasAccess)
    .map(perm => perm.label);
  const hasAmRuleCreateAccess = missingPermissions.length === 0;

  const [editor, setEditor] = useState<AccessMonitoringRuleType>();
  const [plugins, setPlugins] = useState<Plugin[]>([]);
  const [search, setSearch] = useState('');

  const [viewingRule, setViewingRule] =
    useState<AccessMonitoringRuleWithYaml>();

  const showEditor =
    editor === AccessMonitoringRuleType.Notification ||
    editor === AccessMonitoringRuleType.Review;

  const serverSidePagination =
    useServerSidePagination<AccessMonitoringRuleWithYaml>({
      pageSize: 20,
      fetchFunc: async (_, params) => {
        return await fetch(params);
      },
      clusterId: '',
      params: { search },
    });

  const {
    attempt: fetchPluginsAttempt,
    setAttempt: setPluginsAttempt,
    run: pluginRun,
  } = useAttempt('processing');

  function fetchResources() {
    serverSidePagination.fetch();
    if (hasPluginAccess) {
      fetchPlugins();
    } else {
      setPluginsAttempt({ status: 'success' });
    }
  }

  useEffect(() => {
    fetchResources();
  }, []);

  function fetchPlugins() {
    // TODO(lisa): extend as backend support for more plugins
    // is added. Currently only supports slack, msteams and mattermost.
    pluginRun(() =>
      pluginsService.fetchPlugins().then(resp => {
        const filteredPlugins = resp.filter(
          r =>
            r.kind === 'slack' ||
            r.kind === 'mattermost' ||
            r.kind === 'datadog' ||
            r.kind === 'msteams' ||
            r.kind === 'email'
        );
        setPlugins(filteredPlugins);
      })
    );
  }

  function toggleViewingRule(rule: AccessMonitoringRuleWithYaml) {
    if (
      !viewingRule ||
      viewingRule.object.metadata.name != rule.object.metadata.name
    ) {
      setViewingRule(rule);
      if (rule.object.spec.automatic_review) {
        setEditor(AccessMonitoringRuleType.Review);
      } else {
        setEditor(AccessMonitoringRuleType.Notification);
      }
    } else {
      setViewingRule(null);
      setEditor(null);
    }
  }

  async function onDelete() {
    await remove(viewingRule.object.metadata.name);
    serverSidePagination.modifyFetchedData(resp => ({
      ...resp,
      agents: resp.agents.filter(
        rule => rule.object.metadata.name !== viewingRule.object.metadata.name
      ),
    }));
    setViewingRule(null);
    setEditor(null);
  }

  function onCreateRule(editor: AccessMonitoringRuleType) {
    if (showEditor) {
      setViewingRule(null);
    }
    setEditor(editor);
  }

  async function onEdit(editedRule: Partial<AccessMonitoringRuleWithYaml>) {
    const response: AccessMonitoringRuleWithYaml = await update(
      viewingRule.object.metadata.name,
      editedRule
    );
    serverSidePagination.modifyFetchedData(resp => ({
      ...resp,
      agents: resp.agents.map(rule =>
        rule.object.metadata.name === response.object.metadata.name
          ? response
          : rule
      ),
    }));
    setEditor(null);
    setViewingRule(null);
  }

  async function onSave(newRule: Partial<AccessMonitoringRuleWithYaml>) {
    const response: AccessMonitoringRuleWithYaml = await create(newRule);
    serverSidePagination.modifyFetchedData(resp => ({
      ...resp,
      agents: [response, ...resp.agents],
    }));
    setEditor(null);
    setViewingRule(null);
  }

  function handleEditorCancel() {
    setEditor(null);
    setViewingRule(null);
  }

  const [terraformRuleName, setTerraformRuleName] = useState('');

  return (
    <FeatureBox>
      <FeatureHeader alignItems="center" justifyContent="space-between">
        <Flex alignItems="center" mr={3}>
          <FeatureHeaderTitle>Access Automations</FeatureHeaderTitle>
        </Flex>
        <InfoGuideButton config={{ guide: <InfoGuide /> }}>
          <HoverTooltip
            placement="bottom"
            tipContent={
              hasAmRuleCreateAccess ? null : (
                <MissingPermissionsTooltip
                  missingPermissions={missingPermissions}
                />
              )
            }
          >
            <MenuButton
              menuProps={{ menuListCss: () => 'width: 280px' }}
              buttonText="Create New Access Automation"
              buttonProps={{
                width: 280,
                padding: 0,
                size: 'medium',
                intent: 'primary',
                color: 'inherit',
                fill:
                  serverSidePagination.attempt.status === 'success' &&
                  serverSidePagination.fetchedData.agents.length === 0
                    ? 'filled'
                    : 'border',
                disabled:
                  (showEditor && !viewingRule) || !hasAmRuleCreateAccess,
              }}
            >
              <MenuItem
                margin={1}
                px={3}
                borderRadius={2}
                onClick={() =>
                  onCreateRule(AccessMonitoringRuleType.Notification)
                }
              >
                Notification Routing Rule
              </MenuItem>
              <MenuItem
                margin={1}
                px={3}
                borderRadius={2}
                onClick={() => onCreateRule(AccessMonitoringRuleType.Review)}
              >
                Automatic Review Rule
              </MenuItem>
            </MenuButton>
          </HoverTooltip>
        </InfoGuideButton>
      </FeatureHeader>
      {renderAlert(
        serverSidePagination.attempt,
        fetchPluginsAttempt,
        fetchResources
      )}
      <Flex flex="1">
        <Box flex="1" mb="4">
          <RuleList
            serversidePagination={serverSidePagination}
            plugins={plugins}
            onSearchChange={setSearch}
            search={search}
            onEdit={toggleViewingRule}
            onViewTerraform={rule =>
              setTerraformRuleName(rule?.object?.metadata?.name)
            }
            viewingRule={viewingRule}
            rulesAcl={rulesAcl}
          />
        </Box>
        {showEditor && (
          <RuleEditor
            // key creates a new component instance when rule changes
            // instead of updating the mounted component
            key={viewingRule?.object.metadata.name}
            selectedRule={viewingRule}
            onSave={onSave}
            onCancel={handleEditorCancel}
            onEdit={onEdit}
            onDelete={onDelete}
            plugins={plugins}
            editor={editor}
          />
        )}
        {terraformRuleName !== '' && (
          <TerraformDialog
            ruleName={terraformRuleName}
            onCancel={() => setTerraformRuleName('')}
          />
        )}
      </Flex>
    </FeatureBox>
  );
}

function renderAlert(
  rulesAttempt: Attempt,
  pluginsAttempt: Attempt,
  onClick: () => void
): JSX.Element {
  const fetchRulesFailed = rulesAttempt.status === 'failed';
  const fetchPluginsFailed = pluginsAttempt.status === 'failed';
  if (!fetchRulesFailed && !fetchPluginsFailed) return null;

  return (
    <Alert mt={3} primaryAction={{ content: 'Retry', onClick: onClick }}>
      <Box>
        {fetchRulesFailed && (
          <Text>
            Failed to fetch Access Automations: {rulesAttempt.statusText}
          </Text>
        )}
        {fetchPluginsFailed && (
          <Text>Failed to fetch Integrations: {pluginsAttempt.statusText}</Text>
        )}
      </Box>
    </Alert>
  );
}

const infoGuideReferenceLinks = {
  NotificationRoutingRules: {
    title: 'Teleport Notification Routing Rules',
    href: 'https://goteleport.com/docs/identity-governance/access-request-plugins/notification-routing-rules/',
  },
  AutomaticReviewRules: {
    title: 'Teleport Automatic Review Rules',
    href: 'https://goteleport.com/docs/identity-governance/access-requests/automatic-reviews/',
  },
};

function InfoGuide() {
  return (
    <Box>
      <InfoParagraph>
        Teleport Access Automations allow administrators to monitor access
        requests and apply notification routing rules or automatic review rules
        based on specific conditions.
      </InfoParagraph>
      <ReferenceLinks links={Object.values(infoGuideReferenceLinks)} />
    </Box>
  );
}
