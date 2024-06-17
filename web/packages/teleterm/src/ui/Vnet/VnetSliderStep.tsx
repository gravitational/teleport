/**
 * Teleport
 * Copyright (C) 2024 Gravitational, Inc.
 *
 * This program is free software: you can redistribute it and/or modify
 * it under the terms of the GNU Affero General Public License as published by
 * the Free Software Foundation, either version 3 of the License, or
 * (at your option) any later version.
 *
 * This program is distributed in the hope that it will be useful,
 * but WITHOUT ANY WARRANTY; without even the implied warranty of
 * MERCHANTABILITY or FITNESS FOR A PARTICULAR PURPOSE.  See the
 * GNU Affero General Public License for more details.
 *
 * You should have received a copy of the GNU Affero General Public License
 * along with this program.  If not, see <http://www.gnu.org/licenses/>.
 */

import { useCallback } from 'react';
import { StepComponentProps } from 'design/StepSlider';
import { Box, Flex, Text } from 'design';
import { mergeRefs } from 'shared/libs/mergeRefs';
import { useRefAutoFocus } from 'shared/hooks';
import * as whatwg from 'whatwg-url';

import { useStoreSelector } from 'teleterm/ui/hooks/useStoreSelector';

import { useVnetContext } from './vnetContext';
import { VnetSliderStepHeader } from './VnetConnectionItem';

/**
 * VnetSliderStep is the second step of StepSlider used in TopBar/Connections. It is shown after
 * selecting VnetConnectionItem from ConnectionsFilterableList.
 */
export const VnetSliderStep = (props: StepComponentProps) => {
  const visible = props.stepIndex === 1 && props.hasTransitionEnded;
  const { status, startAttempt, stopAttempt } = useVnetContext();
  const autoFocusRef = useRefAutoFocus<HTMLElement>({
    shouldFocus: visible,
  });
  const clusters = useStoreSelector(
    'clustersService',
    useCallback(state => state.clusters, [])
  );
  const rootClusters = [...clusters.values()].filter(
    cluster => !cluster.leaf && cluster.connected
  );
  const rootProxyHostnames = rootClusters.map(
    cluster => new whatwg.URL(`https://${cluster.proxyHost}`).hostname
  );

  return (
    // Padding needs to align with the padding of the previous slider step.
    <Box
      p={2}
      ref={mergeRefs([props.refCallback, autoFocusRef])}
      tabIndex={visible ? 0 : -1}
      css={`
        // Do not show the outline when focused. This element cannot be interacted with and we focus
        // it only so that the next tab press is going to focus the VNet header button instead.
        outline: none;
      `}
    >
      <VnetSliderStepHeader goBack={props.prev} />
      <Flex
        p={textSpacing}
        gap={1}
        flexDirection="column"
        css={`
          &:empty {
            display: none;
          }
        `}
      >
        {startAttempt.status === 'error' && (
          <Text>Could not start VNet: {startAttempt.statusText}</Text>
        )}
        {stopAttempt.status === 'error' && (
          <Text>Could not stop VNet: {stopAttempt.statusText}</Text>
        )}

        {status.value === 'stopped' &&
          (status.reason.value === 'unexpected-shutdown' ? (
            <Text>
              VNet unexpectedly shut down:{' '}
              {status.reason.errorMessage ||
                'no direct reason was given, please check logs'}
              .
            </Text>
          ) : (
            <>
              <Text>
                VNet enables any program to connect to TCP applications
                protected by Teleport.
              </Text>
              <Text>
                Just start VNet and connect to any TCP app over its public
                address – VNet authenticates the connection for you under the
                hood.
              </Text>
            </>
          ))}
      </Flex>

      {status.value === 'running' &&
        (rootClusters.length === 0 ? (
          <Text p={textSpacing}>
            No clusters connected yet, VNet is not proxying any connections.
          </Text>
        ) : (
          <>
            {/* TODO(ravicious): Add leaf clusters and custom DNS zones when support for them
                lands in VNet. */}
            <Text p={textSpacing}>
              Proxying TCP connections to {rootProxyHostnames.join(', ')}
            </Text>
          </>
        ))}
    </Box>
  );
};

const textSpacing = 1;
