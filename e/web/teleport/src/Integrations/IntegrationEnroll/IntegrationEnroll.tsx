import React, { FormEvent } from 'react';

import { useParams } from 'react-router';
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

import { getXCSRFToken } from 'teleport/services/api';

import cfg from 'e-teleport/config';

import { pluginTypeMap } from '../data';

import { State, useIntegrationEnroll } from './useIntegrationEnroll';
import { IntegrationPick } from './IntegrationPick';

export function Container() {
  const state = useIntegrationEnroll();
  return <IntegrationEnroll {...state} />;
}

export function IntegrationEnroll(props: State) {
  const { attempt, availableTypes, existingTypes } = props;
  const { type: selectedType } = useParams<{ type: string }>();

  return (
    <FeatureBox>
      {selectedType && <PluginForm selectedType={selectedType} />}
      {!selectedType && (
        <>
          <FeatureHeader>
            <FeatureHeaderTitle>Select Integration Type</FeatureHeaderTitle>
          </FeatureHeader>
          <Box>
            {attempt.status === 'processing' && (
              <Box textAlign="center" m={10}>
                <Indicator />
              </Box>
            )}
            {attempt.status === 'success' && (
              <IntegrationPick
                availableTypes={availableTypes}
                existingTypes={existingTypes}
              />
            )}
            {attempt.status === 'failed' && (
              <Alert children={attempt.statusText} />
            )}
          </Box>
        </>
      )}
    </FeatureBox>
  );
}

function PluginForm({ selectedType }: { selectedType: string }) {
  const resolvedType = pluginTypeMap[selectedType];
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
        <Flex gap={6} p={4} bg="primary.light">
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
