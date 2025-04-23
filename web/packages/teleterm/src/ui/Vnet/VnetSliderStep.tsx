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

import { PropsWithChildren, useCallback, useEffect, useRef } from 'react';

import { Box, ButtonSecondary, Flex, Text } from 'design';
import { StepComponentProps } from 'design/StepSlider';
import { useRefAutoFocus } from 'shared/hooks';
import { useDelayedRepeatedAttempt } from 'shared/hooks/useAsync';
import { mergeRefs } from 'shared/libs/mergeRefs';

import { ConnectionStatusIndicator } from 'teleterm/ui/TopBar/Connections/ConnectionsFilterableList/ConnectionStatusIndicator';

import { DiagnosticsAlert } from './DiagnosticsAlert';
import { textSpacing } from './sliderStep';
import { VnetSliderStepHeader } from './VnetConnectionItem';
import { useVnetContext } from './vnetContext';

/**
 * VnetSliderStep is the second step of StepSlider used in TopBar/Connections. It is shown after
 * selecting VnetConnectionItem from ConnectionsFilterableList.
 */
export const VnetSliderStep = (props: StepComponentProps) => {
  const visible = props.stepIndex === 1 && props.hasTransitionEnded;
  const {
    status,
    startAttempt,
    stopAttempt,
    runDiagnostics,
    reinstateDiagnosticsAlert,
  } = useVnetContext();
  const autoFocusRef = useRefAutoFocus<HTMLElement>({
    shouldFocus: visible,
  });
  /**
   * If the user has previously dismissed an alert, requesting a manual run from the VNet panel
   * should show it again.
   */
  const runDiagnosticsFromVnetPanel = useCallback(
    () =>
      // Reinstate the alert only after the run has finished. This is so that if there are results
      // from a previous run, we don't show them immediately after the user requests a manual run of
      // diagnostics.
      runDiagnostics().finally(() => reinstateDiagnosticsAlert()),
    [runDiagnostics, reinstateDiagnosticsAlert]
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
      <VnetSliderStepHeader
        goBack={props.prev}
        runDiagnosticsFromVnetPanel={runDiagnosticsFromVnetPanel}
      />
      <Flex
        p={textSpacing}
        gap={3}
        flexDirection="column"
        css={`
          &:empty {
            display: none;
          }
        `}
      >
        {startAttempt.status === 'error' && (
          <ErrorText>Could not start VNet: {startAttempt.statusText}</ErrorText>
        )}
        {stopAttempt.status === 'error' && (
          <ErrorText>Could not stop VNet: {stopAttempt.statusText}</ErrorText>
        )}

        {status.value === 'stopped' &&
          (status.reason.value === 'unexpected-shutdown' ? (
            <ErrorText>
              VNet unexpectedly shut down:{' '}
              {status.reason.errorMessage ||
                'no direct reason was given, please check logs'}
              .
            </ErrorText>
          ) : (
            <Flex flexDirection="column" gap={1}>
              <Text>
                VNet enables any program to connect to TCP apps protected by
                Teleport.
              </Text>
              <Text>
                Start VNet and connect to any TCP app over its public address –
                VNet authenticates the connection for you under the hood.
              </Text>
            </Flex>
          ))}
      </Flex>

      {status.value === 'running' && <DnsZones />}

      <DiagnosticsAlert
        runDiagnosticsFromVnetPanel={runDiagnosticsFromVnetPanel}
      />
    </Box>
  );
};

const ErrorText = (props: PropsWithChildren) => (
  <Text>
    <ConnectionStatusIndicator status="error" inline mr={2} />
    {props.children}
  </Text>
);

/**
 * DnsZones displays the list of currently proxied DNS zones, as understood by the VNet admin
 * process. The list is cached in the context and updated when the VNet panel gets opened.
 *
 * As for 95% of users the list will never change during the lifespan of VNet, the VNet panel always
 * optimistically displays previously fetched results while fetching new list.
 */
const DnsZones = () => {
  const { listDNSZones, listDNSZonesAttempt: eagerListDNSZonesAttempt } =
    useVnetContext();
  const listDNSZonesAttempt = useDelayedRepeatedAttempt(
    eagerListDNSZonesAttempt
  );
  const dnsZonesRefreshRequestedRef = useRef(false);

  useEffect(
    function refreshListOnOpen() {
      if (!dnsZonesRefreshRequestedRef.current) {
        dnsZonesRefreshRequestedRef.current = true;
        listDNSZones();
      }
    },
    [listDNSZones]
  );

  if (listDNSZonesAttempt.status === 'error') {
    return (
      <Text p={textSpacing}>
        <ConnectionStatusIndicator status="warning" inline mr={2} />
        VNet is working, but Teleport Connect could not fetch DNS zones:{' '}
        {listDNSZonesAttempt.statusText}
        <ButtonSecondary
          ml={2}
          size="small"
          type="button"
          onClick={listDNSZones}
        >
          Retry
        </ButtonSecondary>
      </Text>
    );
  }

  if (
    listDNSZonesAttempt.status === '' ||
    (listDNSZonesAttempt.status === 'processing' && !listDNSZonesAttempt.data)
  ) {
    return (
      <Text p={textSpacing}>
        <ConnectionStatusIndicator status="processing" inline mr={2} />
        Updating the list of DNS zones…
      </Text>
    );
  }

  const dnsZones = listDNSZonesAttempt.data;

  return (
    <Text p={textSpacing}>
      <ConnectionStatusIndicator
        status={listDNSZonesAttempt.status === 'success' ? 'on' : 'processing'}
        title={
          listDNSZonesAttempt.status === 'processing'
            ? 'Updating the list of DNS zones…'
            : undefined
        }
        inline
        mr={2}
      />
      {dnsZones.length === 0 ? (
        <>No clusters connected yet, VNet is not proxying any connections.</>
      ) : (
        <>Proxying TCP connections to {dnsZones.join(', ')}</>
      )}
    </Text>
  );
};
