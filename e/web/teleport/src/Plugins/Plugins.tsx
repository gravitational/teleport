import React from 'react';

import { Link } from 'react-router-dom';

import { Indicator, Box, ButtonPrimary, Alert } from 'design';

import {
  FeatureBox,
  FeatureHeader,
  FeatureHeaderTitle,
} from 'teleport/components/Layout';

import cfg from 'e-teleport/config';

import { usePlugins, State } from './usePlugins';
import { PluginList } from './PluginList';
import { PluginDelete } from './PluginDelete';

export default function Container() {
  const state = usePlugins();
  return <Plugins {...state} />;
}

export function Plugins(props: State) {
  const { attempt, items, onCancelDelete, onDelete, onStartDelete, operation } =
    props;

  return (
    <FeatureBox>
      <FeatureHeader>
        <FeatureHeaderTitle>Integrations</FeatureHeaderTitle>
        <Box mr={0} ml="auto">
          <Link to={cfg.getIntegrationEnrollRoute()}>
            <ButtonPrimary width="240px">Enroll new integration</ButtonPrimary>
          </Link>
        </Box>
      </FeatureHeader>
      {attempt.status === 'failed' && <Alert children={attempt.statusText} />}
      {attempt.status === 'processing' && (
        <Box textAlign="center" m={10}>
          <Indicator />
        </Box>
      )}
      {attempt.status == 'success' && (
        /* TODO(justinas): show splash screen when list is empty */
        <PluginList plugins={items} onDelete={onStartDelete} />
      )}
      {operation.type === 'delete' && (
        <PluginDelete
          onClose={onCancelDelete}
          onDelete={() => onDelete(operation.plugin)}
        />
      )}
    </FeatureBox>
  );
}
