import { useMutation } from '@tanstack/react-query';
import React, { PropsWithChildren, useMemo, useState } from 'react';
import { Link, useNavigate, useParams } from 'react-router';

import { Alert, Box, ButtonIcon, Flex, Indicator, Label, Text } from 'design';
import { ArrowLeft } from 'design/Icon';
import { ResourceIcon } from 'design/ResourceIcon';
import { HoverTooltip } from 'design/Tooltip';
import { getErrMessage } from 'shared/utils/errorType';
import { capitalizeFirstLetter } from 'shared/utils/text';

import { pluginsService } from 'e-teleport/services/plugins';
import { useFetchPlugin } from 'e-teleport/services/plugins/hooks';
import { FeatureBox } from 'teleport/components/Layout';
import cfg from 'teleport/config';
import { IntegrationLike } from 'teleport/Integrations/IntegrationList';
import { IntegrationStatus as OSSIntegrationStatus } from 'teleport/Integrations/IntegrationStatus';
import {
  IntegrationStatusCode,
  PluginKind,
} from 'teleport/services/integrations';

import { PluginDelete } from '../PluginDelete';
import { EntraStatusRoutes } from './Entra/EntraStatus';
import { OktaStatusDetails } from './OktaStatusDetails/OktaStatusDetails';
import { OverallStatus } from './Shared';

export function IntegrationStatus() {
  const navigate = useNavigate();

  const { type, name = '' } = useParams<{
    type: PluginKind;
    name: string;
  }>();

  const shouldFetchPlugin = type === 'okta' || type === 'entra-id';

  const plugin = useFetchPlugin(name, {
    enabled: shouldFetchPlugin,
  });

  const deletePlugin = useMutation({
    mutationFn: pluginsService.deletePlugin,
    onSuccess: () => {
      // redirect to integrations page after deletion
      navigate(cfg.routes.integrations);
    },
  });

  const [showDeleteDialog, setShowDeleteDialog] = useState(false);

  function onDelete() {
    if (plugin.data?.name) {
      return deletePlugin.mutateAsync(plugin.data.name);
    }
  }

  const props: FeatureContainerProps = useMemo(
    () => ({
      pluginName: name,
      pluginType: type,
      statusCode: plugin.data?.statusCode,
      status: plugin.data?.status,
    }),
    [name, type, plugin.data?.statusCode, plugin.data?.status]
  );

  if (shouldFetchPlugin) {
    if (plugin.isError) {
      return (
        <FeatureContainer {...props}>
          <Alert>{getErrMessage(plugin.error)}</Alert>
        </FeatureContainer>
      );
    }

    if (plugin.isPending) {
      return (
        <FeatureContainer {...props}>
          <Box textAlign="center" m={10}>
            <Indicator />
          </Box>
        </FeatureContainer>
      );
    }

    if (plugin.isSuccess && plugin.data?.kind === 'okta') {
      return (
        <FeatureContainer {...props}>
          <OktaStatusDetails
            plugin={plugin.data}
            deletePlugin={() => setShowDeleteDialog(true)}
          />
          {showDeleteDialog && (
            <PluginDelete
              onClose={() => setShowDeleteDialog(false)}
              onDelete={onDelete}
              pluginKind={plugin.data.kind}
            />
          )}
        </FeatureContainer>
      );
    }

    if (plugin.isSuccess && plugin.data?.kind === 'entra-id') {
      return (
        <FeatureContainer {...props}>
          <EntraStatusRoutes
            plugin={plugin.data}
            deletePlugin={() => setShowDeleteDialog(true)}
          />
          {showDeleteDialog && (
            <PluginDelete
              onClose={() => setShowDeleteDialog(false)}
              onDelete={onDelete}
              pluginKind={plugin.data.kind}
            />
          )}
        </FeatureContainer>
      );
    }
  }

  return <OSSIntegrationStatus />;
}

type FeatureContainerProps = {
  pluginName: string;
  pluginType: PluginKind;
  statusCode: IntegrationStatusCode;
  status?: IntegrationLike['status'];
};
const FeatureContainer: React.FC<PropsWithChildren<FeatureContainerProps>> = ({
  children,
  pluginName,
  pluginType,
  statusCode,
  status,
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
          <Label kind="secondary" css={{ borderRadius: '999px' }}>
            <Flex py={1} gap={1} alignItems="center">
              {getIcon(pluginType)}
              <Text fontSize={1}>{getSubTitle(pluginType)} Integration</Text>
            </Flex>
          </Label>
        </Flex>
        {statusCode && (
          <Box mr={4}>
            <OverallStatus statusCode={statusCode} status={status} />
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

  if (type === 'entra-id') {
    return <ResourceIcon name="entraid" mr={1} width="20px" height="20px" />;
  }
}

function getSubTitle(kind: PluginKind) {
  if (kind === 'okta') {
    return capitalizeFirstLetter(kind);
  }

  if (kind === 'entra-id') {
    return 'Microsoft Entra ID';
  }
}
