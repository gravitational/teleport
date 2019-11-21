/*
Copyright 2019 Gravitational, Inc.

Licensed under the Apache License, Version 2.0 (the "License");
you may not use this file except in compliance with the License.
You may obtain a copy of the License at

    http://www.apache.org/licenses/LICENSE-2.0

Unless required by applicable law or agreed to in writing, software
distributed under the License is distributed on an "AS IS" BASIS,
WITHOUT WARRANTIES OR CONDITIONS OF ANY KIND, either express or implied.
See the License for the specific language governing permissions and
limitations under the License.
*/

import React from 'react';
import { NavLink } from 'react-router-dom';
import MenuAction, {
  MenuItem,
  MenuItemIcon,
} from 'teleport/components/ActionMenu';
import * as Icons from 'design/Icon';
import { Flex, ButtonIcon, ButtonPrimary } from 'design';
import useConsoleContext, {
  useStoreDialogs,
  SessionStateEnum,
} from './../../useConsoleContext';
import cfg from 'teleport/config';
import history from 'teleport/services/history';

export default function ActionBar({ tabId, onLogout }) {
  const consoleContext = useConsoleContext();
  const { status, url } = consoleContext.storeDocs.find(tabId) || {};
  const isConnected = status === SessionStateEnum.CONNECTED;
  const isHome = url === cfg.getConsoleRoute();

  // subscribe to dialog changes
  const storeDialogs = useStoreDialogs();
  const { isDownloadOpen, isUploadOpen } = storeDialogs.getState(tabId);
  const isScpDisabled = isDownloadOpen || isUploadOpen || !isConnected;

  function onOpenDownalod() {
    storeDialogs.openDownload(tabId);
  }

  function onOpenUpload() {
    storeDialogs.openUpload(tabId);
  }

  function goToHome() {
    history.push(cfg.getConsoleRoute());
  }

  return (
    <Flex alignItems="center">
      <ButtonIcon
        disabled={isHome}
        onClick={goToHome}
        size={0}
        title="New Tab"
        to={cfg.getConsoleRoute()}
      >
        <Icons.Add fontSize="16px" />
      </ButtonIcon>
      <ButtonIcon
        disabled={isScpDisabled}
        size={0}
        title="Download files"
        onClick={onOpenDownalod}
      >
        <Icons.Download fontSize="16px" />
      </ButtonIcon>
      <ButtonIcon
        disabled={isScpDisabled}
        size={0}
        title="Upload files"
        onClick={onOpenUpload}
      >
        <Icons.Upload fontSize="16px" />
      </ButtonIcon>
      <MenuAction
        buttonIconProps={{ mr: 2, ml: 2, size: 0, style: { fontSize: '16px' } }}
        menuProps={menuProps}
      >
        <MenuItem as={NavLink} to={cfg.getClusterRoute()}>
          <MenuItemIcon as={Icons.Home} mr="2" />
          Dashboard
        </MenuItem>
        <MenuItem>
          <ButtonPrimary my={3} block onClick={onLogout}>
            Sign Out
          </ButtonPrimary>
        </MenuItem>
      </MenuAction>
    </Flex>
  );
}

const menuListCss = () => `
  width: 250px;
`;

const menuProps = {
  menuListCss: menuListCss,
  anchorOrigin: {
    vertical: 'center',
    horizontal: 'center',
  },
  transformOrigin: {
    vertical: 'top',
    horizontal: 'center',
  },
};
