import React, { FormEvent } from 'react';

import { Link } from 'react-router-dom';

import styled from 'styled-components';

import {
  Alert,
  ButtonPrimary,
  ButtonSecondary,
  Box,
  Flex,
  Indicator,
  Text,
} from 'design';
import { NotFound } from 'design/CardError';
import { ToolTipInfo } from 'shared/components/ToolTip';
import Validation, { Validator } from 'shared/components/Validation';
import {
  FeatureBox,
  FeatureHeader,
  FeatureHeaderTitle,
} from 'teleport/components/Layout';
import { Switch } from 'teleport/components/Router';
import { getXCSRFToken } from 'teleport/services/api';
import { IntegrationKind } from 'teleport/services/integrations';
import { getRoutesToEnrollIntegrations } from 'teleport/Integrations/Enroll';

import cfg from 'e-teleport/config';

import { PluginType, pluginTypeMap } from '../data';

import { State, useIntegrationEnroll } from './useIntegrationEnroll';
import { IntegrationPick } from './IntegrationPick';
import { PluginEnrollSuccess } from './PluginEnrollSuccess';
import { PluginEnrollFailedDialog } from './PluginEnrollFailedDialog';

export function Container() {
  const state = useIntegrationEnroll();
  return <IntegrationEnroll {...state} />;
}

export function IntegrationEnroll(props: State) {
  const {
    attempt,
    availableTypes,
    existingTypes,
    hasPluginAccess,
    hasIntegrationAccess,
    selectedType,
    enrollResponse,
  } = props;

  if (selectedType) {
    if (selectedType === IntegrationKind.AwsOidc) {
      return (
        <FeatureBox>
          <Switch>{getRoutesToEnrollIntegrations()}</Switch>
        </FeatureBox>
      );
    }

    const resolvedType = pluginTypeMap[selectedType];
    if (!resolvedType || !resolvedType.hosted) {
      return <NotFound message="not found" />;
    }

    // If we're coming back from enrollment flow
    // (there is either a "success" or an "error" result,
    // display the result.
    if (enrollResponse.success) {
      return (
        <PluginEnrollSuccess
          resolvedType={resolvedType}
          success={enrollResponse.success}
        />
      );
    }
    if (enrollResponse.error) {
      return (
        <FeatureBox>
          <PluginEnrollFailedDialog
            resolvedType={resolvedType}
            error={enrollResponse.error}
            errorDescription={enrollResponse.errorDescription}
            clearError={enrollResponse.clearError}
          />
          <PluginForm resolvedType={resolvedType} />
        </FeatureBox>
      );
    }

    return (
      <FeatureBox>
        <PluginForm resolvedType={resolvedType} />
      </FeatureBox>
    );
  }

  let content;
  if (attempt.status === 'processing') {
    content = (
      <Box textAlign="center" m={10}>
        <Indicator />
      </Box>
    );
  } else if (attempt.status === 'failed') {
    content = <Alert children={attempt.statusText} />;
  } else {
    content = (
      <IntegrationPick
        availableTypes={availableTypes}
        existingTypes={existingTypes}
        hasPluginAccess={hasPluginAccess}
        hasIntegrationAccess={hasIntegrationAccess}
      />
    );
  }

  return (
    <FeatureBox>
      <FeatureHeader>
        <FeatureHeaderTitle>Select Integration Type</FeatureHeaderTitle>
      </FeatureHeader>
      <Box>{content}</Box>
    </FeatureBox>
  );
}

function PluginForm({ resolvedType }: { resolvedType: PluginType }) {
  if (!resolvedType || !resolvedType.hosted) {
    return <NotFound message="not found" />;
  }

  function onSubmit(validator: Validator, e: FormEvent) {
    if (!validator.validate()) {
      e.preventDefault();
    }
  }

  return (
    <Box mt={3}>
      <Text fontWeight="bold" typography="h4">
        {resolvedType.fullName}
      </Text>
      {resolvedType.Description && <resolvedType.Description />}
      {resolvedType.permissions?.length && (
        <Flex gap={6} p={4} bg="levels.surface">
          {resolvedType.permissions.map((perm, index) => (
            <Box key={index}>
              <Text fontWeight="bold" typography="h6">
                {perm.category}
              </Text>
              <PermissionList>
                {perm.permissions.map(p => (
                  <PermissionListItem key={`${index}${p.title}`}>
                    {p.title}{' '}
                    {p.description && (
                      <ToolTipInfo>{p.description}</ToolTipInfo>
                    )}
                  </PermissionListItem>
                ))}
              </PermissionList>
            </Box>
          ))}
        </Flex>
      )}
      <Box mt={3}>
        <Validation>
          {/* A "normal" HTTP form is used here instead of an AJAX request,
          since the user needs to be redirected to the OAuth provider after submitting. */}
          {({ validator }) => (
            <form
              action={cfg.getPluginUrl()}
              onSubmit={e => onSubmit(validator, e)}
              method="POST"
            >
              <input type="hidden" name="csrf_token" value={getXCSRFToken()} />
              {/* TODO: make this a proper input and validate against existing instances
          when we start allowing multiple instances per type */}
              <input
                type="hidden"
                name="name"
                value={`${resolvedType.type}-default`}
              />
              <input type="hidden" name="type" value={resolvedType.type} />
              <Flex flexDirection="column">
                <Flex>
                  {resolvedType.FormMixin && <resolvedType.FormMixin />}
                </Flex>
                <Flex gap={2}>
                  <ButtonPrimary width="200px" type="submit">
                    Connect {resolvedType.name}
                  </ButtonPrimary>
                  <Link to={cfg.oss.getIntegrationEnrollRoute(null)}>
                    <ButtonSecondary width="200px">Back</ButtonSecondary>
                  </Link>
                </Flex>
              </Flex>
            </form>
          )}
        </Validation>
      </Box>
    </Box>
  );
}

const PermissionList = styled.ul`
  list-style: none;
  padding: 0;
`;

const PermissionListItem = styled.li`
  margin-top: ${({ theme }) => theme.space[2]}px;
`;
