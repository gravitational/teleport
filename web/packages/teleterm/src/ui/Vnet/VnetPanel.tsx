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

import {
  PropsWithChildren,
  useCallback,
  useEffect,
  useRef,
  useState,
} from 'react';

import { Box, ButtonSecondary, ButtonText, Flex, Stack, Text } from 'design';
import { Info } from 'design/Icon';
import { Timestamp } from 'gen-proto-ts/google/protobuf/timestamp_pb';
import {
  RecentConnection,
  RecentConnectionKind,
} from 'gen-proto-ts/teleport/lib/teleterm/vnet/v1/vnet_service_pb';
import { useRefAutoFocus } from 'shared/hooks';
import { useDelayedRepeatedAttempt } from 'shared/hooks/useAsync';

import { useAppContext } from 'teleterm/ui/appContextProvider';
import { ConnectionKindIndicator } from 'teleterm/ui/TopBar/Connections/ConnectionsFilterableList/ConnectionItem';
import { ConnectionStatusIndicator } from 'teleterm/ui/TopBar/Connections/ConnectionsFilterableList/ConnectionStatusIndicator';

import { DiagnosticsAlert } from './DiagnosticsAlert';
import { NetworkGraph } from './NetworkGraph';
import { textSpacing } from './sliderStep';
import { useVnetContext } from './vnetContext';
import { VnetPanelHeader } from './VnetPanelHeader';

/**
 * VnetPanel is the content shown in the popover opened from the VNet icon in the top bar.
 */
export const VnetPanel = () => {
  const {
    status,
    startAttempt,
    stopAttempt,
    installTimeRequirementsCheck,
    runDiagnostics,
    reinstateDiagnosticsAlert,
  } = useVnetContext();
  const autoFocusRef = useRefAutoFocus<HTMLDivElement>({
    shouldFocus: true,
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
    <Box
      p={2}
      ref={autoFocusRef}
      tabIndex={0}
      css={`
        // Do not show the outline when focused. This element cannot be interacted with and we focus
        // it only so that the next tab press is going to focus the VNet header button instead.
        outline: none;
      `}
    >
      <VnetPanelHeader
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
        {installTimeRequirementsCheck.status === 'failed' && (
          <>
            {installTimeRequirementsCheck.reason.kind ===
              'missing-windows-service' && (
              <ErrorText>
                VNet system service is not installed. <br />
                To use VNet, uninstall Teleport Connect and install it again
                selecting &apos;Anyone who uses this computer&apos; option.
                Administrator privileges will be required.
              </ErrorText>
            )}
            {installTimeRequirementsCheck.reason.kind ===
              'windows-service-version-mismatch' && (
              <ErrorText>
                The VNet system service version does not match the application
                version. <br />
                This can happen if Teleport Connect is installed both per-user
                and per-machine. To use VNet, uninstall both Teleport Connect
                installations and install it again selecting &apos;Anyone who
                uses this computer&apos; option. Administrator privileges will
                be required.
              </ErrorText>
            )}
            {installTimeRequirementsCheck.reason.kind === 'error' && (
              <ErrorText>
                Could not perform VNet installation requirements checks:{' '}
                {installTimeRequirementsCheck.reason.statusText}
              </ErrorText>
            )}
          </>
        )}

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

      {status.value === 'running' && (
        <>
          <VnetStatus />
          <RecentConnectionsList />
          <Box px={textSpacing}>
            <SectionLabel>Configuration</SectionLabel>
          </Box>
        </>
      )}

      <DiagnosticsAlert
        runDiagnosticsFromVnetPanel={runDiagnosticsFromVnetPanel}
      />

      {status.value === 'running' && <SshConfigurationHint />}
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
  const {
    refreshServiceInfoAttempt,
    serviceInfoAttempt: eagerServiceInfoAttempt,
  } = useVnetContext();
  const serviceInfoAttempt = useDelayedRepeatedAttempt(eagerServiceInfoAttempt);
  const serviceInfoRefreshRequestedRef = useRef(false);

  useEffect(
    function refreshListOnOpen() {
      if (!serviceInfoRefreshRequestedRef.current) {
        serviceInfoRefreshRequestedRef.current = true;
        refreshServiceInfoAttempt();
      }
    },
    [refreshServiceInfoAttempt]
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
          onClick={refreshServiceInfoAttempt}
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

  return (
    <Stack px={textSpacing} pt={textSpacing} width="100%">
      <SectionLabel>Network activity</SectionLabel>
      <NetworkGraph />
    </Stack>
  );
};

/**
 * SshConfigurationHint is a low-key, muted hint shown at the very bottom of the
 * VNet panel, reporting whether SSH clients are configured to use VNet. It
 * renders nothing until the service info has loaded. Both states share the same
 * icon-plus-text format: an OK icon when configured, a hint icon (and a way to
 * fix it) when not. Unlike a warning, it doesn't raise a status indicator.
 */
const SshConfigurationHint = () => {
  const { serviceInfoAttempt, openSSHConfigurationModal } = useVnetContext();

  if (serviceInfoAttempt.status !== 'success') {
    return null;
  }
  const serviceInfo = serviceInfoAttempt.data;

  return (
    <Flex px={textSpacing} gap={1} alignItems="flex-start">
      <Info size="small" color="text.muted" mt="2px" />
      <SecondaryText>
        SSH clients are not configured to use VNet.{' '}
        <ButtonText
          size="small"
          onClick={() =>
            openSSHConfigurationModal({
              vnetSSHConfigPath: serviceInfo.vnetSshConfigPath,
            })
          }
          css={`
            padding: 0;
            min-height: 0;
            font: inherit;
            color: inherit;
            text-decoration: underline;
          `}
        >
          Configure
        </ButtonText>
      </SecondaryText>
    </Flex>
  );
};

/**
 * SectionLabel is the small muted heading shown above each section of the
 * running VNet panel (network activity, recent connections).
 */
const SectionLabel = (props: PropsWithChildren) => (
  <Text typography="body3" bold color="text.slightlyMuted">
    {props.children}
  </Text>
);

/**
 * SecondaryText is the shared style for the panel's muted, secondary lines:
 * empty-state placeholders and hints sitting beneath a SectionLabel.
 */
export const SecondaryText = (props: PropsWithChildren) => (
  <Text typography="body3" color="text.slightlyMuted">
    {props.children}
  </Text>
);

/**
 * RecentConnectionsList shows the targets recently connected to through VNet,
 * deduplicated per target and ordered most-recently-connected first. The list
 * is streamed from the VNet service and cleared whenever VNet stops.
 */
const RecentConnectionsList = () => {
  const { recentConnections } = useVnetContext();

  return (
    <Box p={textSpacing}>
      <SectionLabel>Recent connections</SectionLabel>
      {recentConnections.length === 0 ? (
        <SecondaryText>No connections yet.</SecondaryText>
      ) : (
        <Flex flexDirection="column">
          {recentConnections.map(connection => (
            <RecentConnectionRow
              key={[
                connection.kind,
                connection.cluster,
                connection.leafCluster,
                connection.displayName,
              ].join('/')}
              connection={connection}
            />
          ))}
        </Flex>
      )}
    </Box>
  );
};

const RecentConnectionRow = (props: { connection: RecentConnection }) => {
  const { kind, displayName, lastConnected, lastClientProcessPath } =
    props.connection;
  const lastConnectedDate = lastConnected && Timestamp.toDate(lastConnected);
  const openedBy =
    lastClientProcessPath && processDisplayName(lastClientProcessPath);
  const appIcon = useAppIcon(lastClientProcessPath);

  return (
    <Flex alignItems="center" gap={1} minWidth={0}>
      <ConnectionKindIndicator
        css={`
          padding: 0px 6px;
          font-weight: 600;
        `}
      >
        {kindLabel(kind)}
      </ConnectionKindIndicator>
      <Flex flexDirection="column" flex="1" minWidth={0}>
        {openedBy && (
          <Flex
            alignItems="center"
            gap={1}
            minWidth={0}
            justifyContent="space-between"
          >
            <Text
              typography="body3"
              title={displayName}
              css={`
                min-width: 0;
                overflow: hidden;
                text-overflow: ellipsis;
                white-space: nowrap;
              `}
            >
              {displayName}
            </Text>
            <Flex gap={1} alignItems="center">
              {appIcon && (
                <img
                  src={appIcon}
                  alt=""
                  css={`
                    max-height: 32px;
                    object-fit: contain;
                  `}
                />
              )}
              <Stack gap={0}>
                <Text
                  typography="body3"
                  color="text.muted"
                  title={lastClientProcessPath}
                  css={`
                    line-height: 14px;
                    white-space: nowrap;
                  `}
                >
                  {openedBy}
                </Text>
                {lastConnectedDate && (
                  <Text
                    typography="body3"
                    color="text.muted"
                    title={lastConnectedDate.toLocaleString()}
                    css={`
                      line-height: 14px;
                      white-space: nowrap;
                    `}
                  >
                    {formatRelativeShort(lastConnectedDate, new Date())}
                  </Text>
                )}
              </Stack>
            </Flex>
          </Flex>
        )}
      </Flex>
    </Flex>
  );
};

/**
 * appIconCache memoizes resolved icon data URLs by executable path across the
 * lifetime of the renderer. Icons don't change while the app runs, and the
 * recent connections list re-renders on every stream update, so caching avoids
 * repeatedly crossing the IPC boundary for the same program. The empty string
 * (no icon available) is cached too, so unresolved paths aren't retried.
 */
const appIconCache = new Map<string, Promise<string>>();

/**
 * useAppIcon resolves the icon of the local program at the given executable
 * path to a data URL through the main process. It returns an empty string until
 * the icon loads, or permanently if the platform or path yields no icon; the
 * caller renders the icon only when the string is non-empty.
 */
function useAppIcon(path: string | undefined): string {
  const { mainProcessClient } = useAppContext();
  const [icon, setIcon] = useState('');

  useEffect(() => {
    if (!path) {
      setIcon('');
      return;
    }

    let cached = appIconCache.get(path);
    if (!cached) {
      cached = mainProcessClient.getAppIcon(path);
      appIconCache.set(path, cached);
    }

    let canceled = false;
    cached.then(
      dataUrl => {
        if (!canceled) {
          setIcon(dataUrl);
        }
      },
      () => {
        // getAppIcon already logs failures in the main process and resolves to
        // an empty string, so a rejection here is unexpected; drop the cached
        // entry so a later render can retry.
        appIconCache.delete(path);
      }
    );
    return () => {
      canceled = true;
    };
  }, [path, mainProcessClient]);

  return icon;
}

/**
 * formatRelativeShort renders how long ago `date` was in a compact form such as
 * "just now", "5m ago", "2h ago", "3d ago", or "2w ago", picking the largest
 * whole unit that fits. Intl.RelativeTimeFormat is intentionally not used: even
 * its narrowest English style produces "5 min. ago" rather than "5m ago", and
 * it doesn't select the unit on its own.
 */
function formatRelativeShort(date: Date, now: Date): string {
  const seconds = Math.round((now.getTime() - date.getTime()) / 1000);
  if (seconds < 60) {
    return 'just now';
  }
  const minutes = Math.floor(seconds / 60);
  if (minutes < 60) {
    return `${minutes}m ago`;
  }
  const hours = Math.floor(minutes / 60);
  if (hours < 24) {
    return `${hours}h ago`;
  }
  const days = Math.floor(hours / 24);
  if (days < 7) {
    return `${days}d ago`;
  }
  const weeks = Math.floor(days / 7);
  return `${weeks}w ago`;
}

/**
 * processDisplayName returns a human-friendly name for a local process
 * executable path, used to show which program opened a connection.
 *
 * For macOS app bundles it uses the name of the outermost .app bundle, e.g.
 * "Google Chrome" for a deeply nested helper executable
 * (".../Google Chrome.app/.../Google Chrome Helper.app/Contents/MacOS/..."),
 * rather than the executable's own basename ("Google Chrome Helper"). This
 * mirrors the icon lookup in the main process' getAppIcon handler. Otherwise it
 * uses the executable's basename with its first letter capitalized, e.g. "Curl"
 * for "/usr/bin/curl".
 */
function processDisplayName(path: string): string {
  if (!path) {
    return '';
  }
  const appBundle = path.match(/^(?:.*?\/)?([^/]+)\.app(?:\/|$)/);
  if (appBundle) {
    return appBundle[1];
  }
  const segments = path.split('/');
  const segment = segments[segments.length - 1] || path;
  return segment[0].toUpperCase() + segment.substring(1);
}

function kindLabel(kind: RecentConnectionKind): string {
  switch (kind) {
    case RecentConnectionKind.APP:
      return 'TCP';
    case RecentConnectionKind.SSH:
      return 'SSH';
    case RecentConnectionKind.DATABASE:
      return 'DB';
    default:
      return '';
  }
}
