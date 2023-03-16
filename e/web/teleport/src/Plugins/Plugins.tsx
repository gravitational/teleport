import React from 'react';
import { Indicator, Box, ButtonPrimary, Alert } from 'design';

import {
  FeatureBox,
  FeatureHeader,
  FeatureHeaderTitle,
} from 'teleport/components/Layout';

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
        {/* TODO(justinas): actually link to "new integration" wizard */}
        <ButtonPrimary ml="auto" width="240px">
          Enroll new integration
        </ButtonPrimary>
      </FeatureHeader>
      {attempt.status === 'failed' && <Alert children={attempt.statusText} />}
      {attempt.status === 'processing' && (
        <Box textAlign="center" m={10}>
          <Indicator />
        </Box>
      )}
      {attempt.status === 'success' && (
        /* TODO(justinas): redirect to "new integration" when 'items' empty*/
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
