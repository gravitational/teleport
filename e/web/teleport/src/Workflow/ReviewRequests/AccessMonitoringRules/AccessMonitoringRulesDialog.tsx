import { forwardRef, JSX, useEffect, useState } from 'react';
import type { TransitionStatus } from 'react-transition-group';
import { useTheme } from 'styled-components';

import { Alert, Box, ButtonIcon, Flex, Text } from 'design';
import Dialog from 'design/Dialog';
import { Cross } from 'design/Icon';
import { Theme } from 'design/theme/themes/types';
import { HoverTooltip } from 'design/Tooltip';
import { MenuButton, MenuItem } from 'shared/components/MenuAction';
import { MissingPermissionsTooltip } from 'shared/components/MissingPermissionsTooltip';
import useAttempt, { Attempt } from 'shared/hooks/useAttemptNext';

import {
  AccessMonitoringRuleType,
  AccessMonitoringRuleWithYaml,
} from 'e-teleport/services/accessmonitoringrule/types';
import { pluginsService } from 'e-teleport/services/plugins';
import { useServerSidePagination } from 'teleport/components/hooks';
import { Plugin } from 'teleport/services/integrations';
import useTeleport from 'teleport/useTeleport';

import { RuleEditor } from './RuleEditor/RuleEditor';
import { RuleList } from './RuleList/RuleList';
import { useRules } from './useRules';

export const AccessMonitoringRulesDialog = forwardRef<
  HTMLDivElement,
  {
    onClose(): void;
    transitionState: TransitionStatus;
  }
>(({ onClose, transitionState }, ref) => {
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

  const theme = useTheme();
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

  return (
    <Dialog
      dialogCss={() => fullScreenDialogCss(theme)}
      disableEscapeKeyDown={false}
      open={true}
      ref={ref}
      className={transitionState}
    >
      <Flex css={{ flex: 1 }} width="100%">
        <Box
          p={4}
          css={`
            overflow: auto;
            position: relative;
            right: 0;
            width: 100%;
          `}
        >
          <Flex alignItems="center" mb={3} justifyContent="space-between">
            <Flex alignItems="center" mr={3}>
              <HoverTooltip
                placement="bottom"
                tipContent="Back to Access Requests"
              >
                <ButtonIcon onClick={onClose} mr={2} ml={'-8px'}>
                  <Cross size="medium" />
                </ButtonIcon>
              </HoverTooltip>
              <Text typography="h1">Access Automation Rules</Text>
            </Flex>
            {fetchPluginsAttempt.status === 'success' && (
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
                  menuProps={{ menuListCss }}
                  buttonText="Create New Access Automation Rule"
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
                    onClick={() =>
                      onCreateRule(AccessMonitoringRuleType.Review)
                    }
                  >
                    Automatic Review Rule
                  </MenuItem>
                </MenuButton>
              </HoverTooltip>
            )}
          </Flex>
          {renderAlert(
            serverSidePagination.attempt,
            fetchPluginsAttempt,
            fetchResources
          )}
          <RuleList
            serversidePagination={serverSidePagination}
            plugins={plugins}
            onSearchChange={setSearch}
            search={search}
            onEdit={toggleViewingRule}
            viewingRule={viewingRule}
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
      </Flex>
    </Dialog>
  );
});

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
            Failed to fetch Access Automation Rules: {rulesAttempt.statusText}
          </Text>
        )}
        {fetchPluginsFailed && (
          <Text>Failed to fetch Integrations: {pluginsAttempt.statusText}</Text>
        )}
      </Box>
    </Alert>
  );
}

const menuListCss = () => `
  width: 280px;
`;

const fullScreenDialogCss = (theme: Theme) => {
  return `
  padding: 0;
  width: 100%;
  height: 100%;
  max-height: 100%;
  right: 0;
  border-radius: 0;
  overflow-y: hidden;
  flex-direction: row;
  background: ${theme.colors.levels.sunken};
  transition: width 300ms ease-out;


  &.entering {
    right: -100%;
  }

  &.entered {
    right: 0px;
    transition: right 300ms ease-out;
  }

  &.exiting {
    right: -100%;
    transition: right 300ms ease-out;
  }

  &.exited {
    right: -100%;
  }
  `;
};
