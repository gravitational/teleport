import { forwardRef, useCallback, useEffect, useState } from 'react';
import type { TransitionStatus } from 'react-transition-group';
import { useTheme } from 'styled-components';

import { Alert, Box, ButtonIcon, Flex, Indicator, Text } from 'design';
import Dialog from 'design/Dialog';
import { Cross } from 'design/Icon';
import { Theme } from 'design/theme/themes/types';
import { HoverTooltip } from 'design/Tooltip';
import { MenuButton, MenuItem } from 'shared/components/MenuAction';
import { MissingPermissionsTooltip } from 'shared/components/MissingPermissionsTooltip';
import useAttempt from 'shared/hooks/useAttemptNext';
import { useKeyBasedPagination } from 'shared/hooks/useInfiniteScroll';

import { accessMonitoringRuleService } from 'e-teleport/services/accessmonitoringrule';
import {
  AccessMonitoringRule,
  AccessMonitoringRuleType,
  AccessMonitoringRuleWithYaml,
} from 'e-teleport/services/accessmonitoringrule/types';
import { pluginsService } from 'e-teleport/services/plugins';
import { Plugin } from 'teleport/services/integrations';
import useStickyClusterId from 'teleport/useStickyClusterId';
import useTeleport from 'teleport/useTeleport';

import { AccessMonitoringRuleList } from './AccessMonitoringRuleList';
import { RuleEditor } from './RuleEditor/RuleEditor';

export const AccessMonitoringRulesDialog = forwardRef<
  HTMLDivElement,
  {
    onClose(): void;
    transitionState: TransitionStatus;
  }
>(({ onClose, transitionState }, ref) => {
  const ctx = useTeleport();
  const pluginAccess = ctx.storeUser.getPluginsAccess();
  const hasPluginAccess = pluginAccess.read;
  const amRuleAccess = ctx.storeUser.getAccessMonitoringRuleAccess();
  const missingPermissions = [
    { hasAccess: amRuleAccess.create, label: 'access_monitoring_rule.create' },
    { hasAccess: hasPluginAccess, label: 'plugin.read' },
  ]
    .filter(perm => !perm.hasAccess)
    .map(perm => perm.label);
  const hasAmRuleCreateAccess = missingPermissions.length === 0;

  const theme = useTheme();
  const { clusterId } = useStickyClusterId();
  const [editor, setEditor] = useState<AccessMonitoringRuleType>();
  const [plugins, setPlugins] = useState<Plugin[]>([]);

  const [viewingRule, setViewingRule] =
    useState<AccessMonitoringRuleWithYaml>();

  const showEditor =
    editor === AccessMonitoringRuleType.Notification ||
    editor === AccessMonitoringRuleType.Review;

  const {
    attempt: fetchPluginsAttempt,
    setAttempt: setPluginsAttempt,
    run: pluginRun,
  } = useAttempt('processing');

  const fetchRulesFunc = useCallback(async (params, signal) => {
    const response =
      await accessMonitoringRuleService.fetchAccessMonitoringRulesForAccessRequests(
        clusterId,
        {
          startKey: params.startKey || undefined,
          limit: params.limit,
        },
        signal
      );
    return response;
  }, []);

  const {
    fetch: fetchRules,
    resources: rules,
    attempt: fetchRulesAttempt,
    updateFetchedResources,
  } = useKeyBasedPagination<AccessMonitoringRuleWithYaml>({
    fetchFunc: fetchRulesFunc,
    initialFetchSize: 30,
    fetchMoreSize: 30,
    dataKey: 'rules',
  });

  useEffect(() => {
    if (hasPluginAccess) {
      fetchPlugins();
    } else {
      setPluginsAttempt({ status: 'success' });
    }
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

  function onDelete(deletedRule: AccessMonitoringRule) {
    updateFetchedResources(
      rules.filter(r => r.object.metadata.name !== deletedRule.metadata.name)
    );
    setViewingRule(null);
    setEditor(null);
  }

  function onCreateRule(editor: AccessMonitoringRuleType) {
    if (showEditor) {
      setViewingRule(null);
    }
    setEditor(editor);
  }

  function onEdit(editedRule: AccessMonitoringRuleWithYaml) {
    const index = rules.findIndex(
      a => a.object.metadata.name === editedRule.object.metadata.name
    );
    if (index >= 0) {
      const newResources = [...rules];
      newResources[index] = editedRule;
      updateFetchedResources(newResources);
    } else {
      updateFetchedResources([...rules, editedRule]);
    }
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
                position="bottom"
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
                position="bottom"
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
                      fetchRulesAttempt.status === 'success' &&
                      rules.length === 0
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
          {fetchPluginsAttempt.status === 'failed' && (
            <Alert
              mt={3}
              primaryAction={{ content: 'Retry', onClick: fetchPlugins }}
            >
              <Flex alignItems="center">
                <Text>{fetchPluginsAttempt.statusText}</Text>
              </Flex>
            </Alert>
          )}
          {fetchPluginsAttempt.status === 'success' && (
            <AccessMonitoringRuleList
              attempt={fetchRulesAttempt}
              fetch={fetchRules}
              rules={rules}
              viewingRule={viewingRule?.object}
              toggleViewingRule={toggleViewingRule}
              plugins={plugins}
            />
          )}
          {(fetchRulesAttempt.status === 'processing' ||
            fetchPluginsAttempt.status === 'processing') && (
            <Flex justifyContent="center">
              <Indicator />
            </Flex>
          )}
        </Box>
        {showEditor && (
          <RuleEditor
            // key creates a new component instance when rule changes
            // instead of updating the mounted component
            key={viewingRule?.object.metadata.name}
            selectedRule={viewingRule}
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
