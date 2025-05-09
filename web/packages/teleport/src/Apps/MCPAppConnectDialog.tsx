/*
 * Teleport
 * Copyright (C) 2025  Gravitational, Inc.
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
import { NewTab } from 'design/Icon';
import { getPlatform, Platform } from 'design/platform';
import { SlideTabs, TabSpec } from 'design/SlideTabs/SlideTabs';
import { TextSelectCopy } from 'shared/components/TextSelectCopy';
import { makeDeepLinkWithSafeInput } from 'shared/deepLinks';

import {
    DownloadConnect,
    getConnectDownloadLinks,
} from 'teleport/components/DownloadConnect/DownloadConnect';
import { generateTshLoginCommand } from 'teleport/lib/util';
import { App } from 'teleport/services/apps';
import useStickyClusterId from 'teleport/useStickyClusterId';
import useTeleport from 'teleport/useTeleport';

export function MCPAppConnectDialog(props: { app: App; onClose: () => void }) {
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
            { key: tshId, controls: tshId, title: 'Using tsh' },
        ];
        if (!doesPlatformSupportVnet) {
            tabs.reverse();
        }
        return tabs;
    }, [doesPlatformSupportVnet]);
    const activeTabKey =
        typeof tabs[activeTabIndex] !== 'string' && tabs[activeTabIndex].key;

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
                <Box>
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
                </Box>

                <br />
                <Box>
                <Text>
                    <Text bold as="span">
                        Step 2
                    </Text>
                    {' - Log in the MCP server'}
                </Text>
                <TextSelectCopy text={`tsh mcp login ${app.name}`} />
                </Box>
                <Box>
                    Restart your AI client to load the updated configuration if necessary.
                </Box>
            </DialogContent>

            <DialogFooter>
                <ButtonSecondary onClick={props.onClose}>Close</ButtonSecondary>
            </DialogFooter>
        </Dialog>
    );
}

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
