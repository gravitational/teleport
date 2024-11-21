import React, { PropsWithChildren, useEffect, useState } from 'react';
import { Link, useHistory } from 'react-router-dom';
import { Alert, Box, ButtonIcon, Flex, Indicator, Label, Text } from 'design';
import useAttempt from 'shared/hooks/useAttemptNext';
import { useParams } from 'react-router';
import { ArrowLeft } from 'design/Icon';

import { FeatureBox } from 'teleport/components/Layout';
import {
  IntegrationStatusCode,
  Plugin,
  PluginKind,
} from 'teleport/services/integrations';
import { IntegrationStatus as OSSIntegrationStatus } from 'teleport/Integrations/IntegrationStatus';
import cfg from 'teleport/config';
import { HoverTooltip } from 'shared/components/ToolTip';
import { ResourceIcon } from 'design/ResourceIcon';
import { capitalizeFirstLetter } from 'shared/utils/text';

import { pluginsService } from 'e-teleport/services/plugins';

import { PluginDelete } from '../PluginDelete';

import { OktaStatusDetails } from './OktaStatusDetails/OktaStatusDetails';
import { OverallStatus } from './Shared';

export function IntegrationStatus() {
  const history = useHistory();
  const { attempt, run, setAttempt } = useAttempt('processing');
  const { type, name } = useParams<{
    type: PluginKind;
    name: string;
  }>();

  const [plugin, setPlugin] = useState<Plugin>();
  const [showDeleteDialog, setShowDeleteDialog] = useState(false);

  useEffect(() => {
    if (type === 'okta') {
      run(() => pluginsService.fetchPlugin(name).then(setPlugin));
    } else {
      // If type is not supported in enterprise, we clear the attempt and default to the OSS Integration Status.
      setAttempt({
        status: 'success',
        statusText: undefined,
      });
    }
  }, []);

  function onDelete() {
    return pluginsService.deletePlugin(plugin.name).then(() => {
      // redirect to integrations page after deletion
      history.push(cfg.routes.integrations);
    });
  }

  const props: FeatureContainerProps = {
    pluginName: name,
    pluginType: type,
    statusCode: plugin?.statusCode,
  };

  if (attempt.status === 'failed') {
    return (
      <FeatureContainer {...props}>
        <Alert children={attempt.statusText} />
      </FeatureContainer>
    );
  }

  if (attempt.status === 'processing') {
    return (
      <FeatureContainer {...props}>
        <Box textAlign="center" m={10}>
          <Indicator />
        </Box>
      </FeatureContainer>
    );
  }

  if (plugin?.kind === 'okta') {
    return (
      <FeatureContainer {...props}>
        <OktaStatusDetails
          plugin={plugin}
          deletePlugin={() => setShowDeleteDialog(true)}
        />
        {showDeleteDialog && (
          <PluginDelete
            onClose={() => setShowDeleteDialog(false)}
            onDelete={onDelete}
            pluginKind={plugin.kind}
          />
        )}
      </FeatureContainer>
    );
  }

  return <OSSIntegrationStatus />;
}

type FeatureContainerProps = {
  pluginName: string;
  pluginType: PluginKind;
  statusCode: IntegrationStatusCode;
};
const FeatureContainer: React.FC<PropsWithChildren<FeatureContainerProps>> = ({
  children,
  pluginName,
  pluginType,
  statusCode,
}) => {
  return (
    <FeatureBox css={{ maxWidth: '1400px', paddingTop: '16px' }}>
      <Flex alignItems="center" justifyContent="space-between" mb={3}>
        <Flex alignItems="center">
          <HoverTooltip position="bottom" tipContent="Back to Integrations">
            <ButtonIcon as={Link} to={cfg.routes.integrations} mr={2}>
              <ArrowLeft size="medium" />
            </ButtonIcon>
          </HoverTooltip>
          <Text bold fontSize={6} mr={2}>
            {pluginName}
          </Text>
          <Label kind="secondary">
            <Flex py={1} gap={1} alignItems="center">
              {getIcon(pluginType)}
              <Text fontSize={1}>
                {capitalizeFirstLetter(pluginType)} Integration
              </Text>
            </Flex>
          </Label>
        </Flex>
        {statusCode && (
          <Box mr={4}>
            <OverallStatus statusCode={statusCode} />
          </Box>
        )}
      </Flex>
      {children}
    </FeatureBox>
  );
};

function getIcon(type: PluginKind) {
  if (type === 'okta') {
    return <ResourceIcon name="okta" mr={1} width="20px" height="20px" />;
  }
}
