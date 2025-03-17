/**
 * Teleport
 * Copyright (C) 2025 Gravitational, Inc.
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

import { useMemo, useState } from 'react';
import styled from 'styled-components';

import { Box, ButtonSecondary, Link, Stack, Text } from 'design';
import Dialog, {
  DialogContent,
  DialogFooter,
  DialogHeader,
  DialogTitle,
} from 'design/Dialog';
import { getPlatform, Platform } from 'design/platform';
import { SlideTabs, TabSpec } from 'design/SlideTabs/SlideTabs';
import {
  DownloadConnect,
  getConnectDownloadLinks,
} from 'shared/components/DownloadConnect/DownloadConnect';
import { TextSelectCopy } from 'shared/components/TextSelectCopy';
import { makeDeepLinkWithSafeInput } from 'shared/deepLinks';

import { generateTshLoginCommand } from 'teleport/lib/util';
import { App } from 'teleport/services/apps';
import useStickyClusterId from 'teleport/useStickyClusterId';
import useTeleport from 'teleport/useTeleport';

export function TcpAppConnectDialog(props: { app: App; onClose: () => void }) {
  const { app } = props;
  const ctx = useTeleport();
  const { clusterId } = useStickyClusterId();
  const { cluster, username, authType } = ctx.storeUser.state;
  const accessRequestId = ctx.storeUser.getAccessRequestId();

  // Download button.
  const platform = getPlatform();
  const downloadLinks = getConnectDownloadLinks(platform, cluster.proxyVersion);
  const doesPlatformSupportVnet =
    platform === Platform.macOS || platform === Platform.Windows;

  // Tabs.
  const [activeTabIndex, setActiveTabIndex] = useState(0);
  const tabs: TabSpec[] = useMemo(() => {
    const tabs = [
      {
        key: vnetId,
        controls: vnetId,
        title: doesPlatformSupportVnet
          ? 'Using VNet'
          : 'Using VNet (Windows & macOS only)',
      },
      { key: tshId, controls: tshId, title: 'Using tsh' },
    ];
    if (!doesPlatformSupportVnet) {
      tabs.reverse();
    }
    return tabs;
  }, [doesPlatformSupportVnet]);
  const activeTabKey =
    typeof tabs[activeTabIndex] !== 'string' && tabs[activeTabIndex].key;

  const $vnetSection = (
    <Stack gap={3} width="100%">
      <Text>
        {doesPlatformSupportVnet ? (
          <Link
            href="https://goteleport.com/docs/connect-your-client/vnet/"
            target="_blank"
          >
            VNet
          </Link>
        ) : (
          // If the platform does not support VNet, we already show a "Learn More" button that links
          // to the docs, so let's not link to them twice.
          'VNet'
        )}{' '}
        automatically proxies connections from your computer to TCP apps
        available through Teleport. Any program on your device can connect to an
        application behind Teleport.
      </Text>

      {doesPlatformSupportVnet ? (
        <>
          <Stack>
            <Text>
              <Text bold as="span">
                Step 1
              </Text>
              {' - Download Teleport Connect'}
            </Text>
            <DownloadConnect downloadLinks={downloadLinks} />
          </Stack>

          <Stack>
            <Text>
              <Text bold as="span">
                Step 2
              </Text>
              {' - Start VNet in Teleport Connect'}
            </Text>

            <ButtonSecondary
              as="a"
              href={makeDeepLinkWithSafeInput({
                proxyHost: cluster.publicURL,
                username,
                path: '/vnet',
                searchParams: {},
              })}
            >
              Sign In & Start VNet
            </ButtonSecondary>
          </Stack>

          <Stack fullWidth>
            <Text>
              <Text bold as="span">
                Step 3
              </Text>
              {' - Connect directly to the app'}
            </Text>

            <TextSelectCopy text={app.publicAddr} bash={false} />
          </Stack>
        </>
      ) : (
        // If the platform doesn't support VNet, don't show step-by-step instructions. We don't want
        // the user to follow them and only realize at the very end that the instructions are not for
        // their platform.
        <ButtonSecondary
          as="a"
          href="https://goteleport.com/docs/connect-your-client/vnet/"
          target="_blank"
        >
          Learn More
        </ButtonSecondary>
      )}
    </Stack>
  );

  const $tshSection = (
    <Stack gap={3} fullWidth>
      <Stack fullWidth>
        <Text>
          <Text bold as="span">
            Step 1
          </Text>
          {' - Log in to Teleport'}
        </Text>

        <TextSelectCopy
          text={generateTshLoginCommand({
            authType,
            username,
            clusterId,
            accessRequestId,
          })}
        />
      </Stack>

      <Stack fullWidth>
        <Text>
          <Text bold as="span">
            Step 2
          </Text>
          {' - Start a local proxy and connect to the app'}
        </Text>

        <TextSelectCopy text={`tsh proxy app ${app.name}`} />
      </Stack>

      <Stack>
        <Text>
          <Text bold as="span">
            Step 3
          </Text>
          {
            ' - Connect to the app through the localhost port used by the local proxy'
          }
        </Text>
      </Stack>
    </Stack>
  );

  return (
    <Dialog
      dialogCss={() => ({
        maxWidth: '600px',
        width: '100%',
      })}
      disableEscapeKeyDown={false}
      onClose={props.onClose}
      open={true}
    >
      <DialogHeader mb={4}>
        <DialogTitle>Connect to: {app.name}</DialogTitle>
      </DialogHeader>

      <DialogContent>
        <Stack gap={3}>
          <Box alignSelf="center">
            <SlideTabs
              tabs={tabs}
              activeIndex={activeTabIndex}
              onChange={setActiveTabIndex}
            />
          </Box>

          <TabContentContainer>
            <TabContent id={vnetId} isActive={activeTabKey === vnetId}>
              {$vnetSection}
            </TabContent>

            <TabContent id={tshId} isActive={activeTabKey === tshId}>
              {$tshSection}
            </TabContent>
          </TabContentContainer>
        </Stack>
      </DialogContent>

      <DialogFooter>
        <ButtonSecondary onClick={props.onClose}>Close</ButtonSecondary>
      </DialogFooter>
    </Dialog>
  );
}

const vnetId = 'vnet';
const tshId = 'tsh';

const TabContentContainer = styled(Box)`
  display: grid;
  // Set a min width of 0 to prevent grid items from overflowing the parent.
  // https://css-tricks.com/preventing-a-grid-blowout/
  grid-template-columns: minmax(0, 1fr);
  grid-template-rows: 1fr;
`;

const TabContent = styled(Box).attrs({ role: 'tabpanel' })<{
  isActive: boolean;
}>`
  // All grid items are going to occupy the same cell, but not-active elements are going to have
  // visibility set to hidden. This way the total height of the modal doesn't change when switching
  // between tabs, keeping the position of the modal constant.
  grid-area: 1 / 1;
  ${props => !props.isActive && `visibility: hidden;`}
`;
