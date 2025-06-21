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

import { ConnectionKindIndicator } from 'teleterm/ui/TopBar/Connections/ConnectionsFilterableList/ConnectionItem';
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
                VNet enables any program to connect to TCP apps or SSH servers
                protected by Teleport.
              </Text>
              <Text>
                Start VNet and connect to any TCP app or SSH server at its own
                DNS address – VNet authenticates the connection for you under
                the hood.
              </Text>
            </Flex>
          ))}
      </Flex>

      {status.value === 'running' && <VnetStatus />}

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
 * VnetStatus displays the status of the running VNet service. The list is cached in the context and
 * updated when the VNet panel gets opened.
 *
 * As for 95% of users the list will never change during the lifespan of VNet, the VNet panel always
 * optimistically displays previously fetched results while fetching new list.
 */
const VnetStatus = () => {
  const { getServiceInfo, serviceInfoAttempt: eagerServiceInfoAttempt } =
    useVnetContext();
  const serviceInfoAttempt = useDelayedRepeatedAttempt(eagerServiceInfoAttempt);
  const serviceInfoRefreshRequestedRef = useRef(false);

  useEffect(
    function refreshListOnOpen() {
      if (!serviceInfoRefreshRequestedRef.current) {
        serviceInfoRefreshRequestedRef.current = true;
        getServiceInfo();
      }
    },
    [getServiceInfo]
  );

  if (serviceInfoAttempt.status === 'error') {
    return (
      <Text p={textSpacing}>
        <ConnectionStatusIndicator status="warning" inline mr={2} />
        VNet is running, but Teleport Connect could not fetch its status:{' '}
        {serviceInfoAttempt.statusText}
        <ButtonSecondary
          ml={2}
          size="small"
          type="button"
          onClick={getServiceInfo}
        >
          Retry
        </ButtonSecondary>
      </Text>
    );
  }

  if (
    serviceInfoAttempt.status === '' ||
    (serviceInfoAttempt.status === 'processing' && !serviceInfoAttempt.data)
  ) {
    return (
      <Text p={textSpacing}>
        <ConnectionStatusIndicator status="processing" inline mr={2} />
        Updating VNet status…
      </Text>
    );
  }

  const statusIndicator = (
    <ConnectionStatusIndicator
      status={serviceInfoAttempt.status === 'success' ? 'on' : 'processing'}
      title={
        serviceInfoAttempt.status === 'processing'
          ? 'Updating VNet status…'
          : undefined
      }
      inline
      mr={2}
      mt={2}
    />
  );

  const serviceInfo = serviceInfoAttempt.data;

  const sshConfiguredIndicator = serviceInfo.sshConfigured ? null : (
    <Flex>
      <ConnectionStatusIndicator status={'warning'} inline mr={2} />
      SSH clients are not configured to use VNet (see diag report).
    </Flex>
  );

  if (serviceInfo.appDnsZones.length == 0 && serviceInfo.clusters.length == 0) {
    return (
      <Flex p={textSpacing}>
        {statusIndicator}
        No clusters connected yet, VNet is not proxying any connections.
      </Flex>
    );
  }

  const appDNSZones = new Set(serviceInfo.appDnsZones);
  const sshClusters = new Set(serviceInfo.clusters);
  const appAndSshAreEqual =
    appDNSZones.size == sshClusters.size && appDNSZones.isSubsetOf(sshClusters);

  if (appAndSshAreEqual) {
    return (
      <Text p={textSpacing}>
        <Flex>
          {statusIndicator}
          Proxying TCP and SSH connections to {[...appDNSZones].join(', ')}
        </Flex>
        {sshConfiguredIndicator}
      </Text>
    );
  }

  const both = [...appDNSZones.intersection(sshClusters)].sort();
  const justTCP = [...appDNSZones.difference(sshClusters)].sort();
  const justSSH = [...sshClusters.difference(appDNSZones)].sort();

  return (
    <Box p={textSpacing}>
      <Flex>
        {statusIndicator}
        <Box>
          Proxying TCP and SSH connections to:
          <Text typography="body2">
            {both.length ? (
              <Box>
                <ConnectionKindIndicator bold>TCP</ConnectionKindIndicator>
                <ConnectionKindIndicator bold>SSH</ConnectionKindIndicator>
                {both.join(', ')}
              </Box>
            ) : null}
            {justTCP.length ? (
              <Box>
                <ConnectionKindIndicator bold>TCP</ConnectionKindIndicator>
                {justTCP.join(', ')}
              </Box>
            ) : null}
            {justSSH.length ? (
              <Box>
                <ConnectionKindIndicator bold>SSH</ConnectionKindIndicator>
                {justSSH.join(', ')}
              </Box>
            ) : null}
          </Text>
        </Box>
      </Flex>
      {sshConfiguredIndicator}
    </Box>
  );
};
