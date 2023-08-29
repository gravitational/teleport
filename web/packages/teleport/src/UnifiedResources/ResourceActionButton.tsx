/**
 * Copyright 2023 Gravitational, Inc
 *
 * Licensed under the Apache License, Version 2.0 (the "License");
 * you may not use this file except in compliance with the License.
 * You may obtain a copy of the License at
 *
 *      http://www.apache.org/licenses/LICENSE-2.0
 *
 * Unless required by applicable law or agreed to in writing, software
 * distributed under the License is distributed on an "AS IS" BASIS,
 * WITHOUT WARRANTIES OR CONDITIONS OF ANY KIND, either express or implied.
 * See the License for the specific language governing permissions and
 * limitations under the License.
 */

import React, { useState } from 'react';
import { ButtonBorder } from 'design';
import { LoginItem, MenuLogin } from 'shared/components/MenuLogin';

import { UnifiedResource } from 'teleport/services/agents';
import cfg from 'teleport/config';

import AwsLaunchButton from 'teleport/Apps/AppList/AwsLaunchButton';
import useTeleport from 'teleport/useTeleport';
import { Database } from 'teleport/services/databases';
import { openNewTab } from 'teleport/lib/util';
import { Kube } from 'teleport/services/kube';
import { Desktop } from 'teleport/services/desktops';
import DbConnectDialog from 'teleport/Databases/ConnectDialog';
import KubeConnectDialog from 'teleport/Kubes/ConnectDialog';
import useStickyClusterId from 'teleport/useStickyClusterId';
import { Node } from 'teleport/services/nodes';
import { App } from 'teleport/services/apps';

type Props = {
  resource: UnifiedResource;
};

export const ResourceActionButton = ({ resource }: Props) => {
  switch (resource.kind) {
    case 'node':
      return <NodeConnect node={resource} />;
    case 'app':
      return <AppLaunch app={resource} />;
    case 'db':
      return <DatabaseConnect database={resource} />;
    case 'kube_cluster':
      return <KubeConnect kube={resource} />;
    case 'windows_desktop':
      return <DesktopConnect desktop={resource} />;
    default:
      return null;
  }
};

const NodeConnect = ({ node }: { node: Node }) => {
  const { clusterId } = useStickyClusterId();
  const startSshSession = (login: string, serverId: string) => {
    const url = cfg.getSshConnectRoute({
      clusterId,
      serverId,
      login,
    });

    openNewTab(url);
  };

  function handleOnOpen() {
    return makeNodeOptions(clusterId, node);
  }

  const handleOnSelect = (e: React.SyntheticEvent, login: string) => {
    e.preventDefault();
    return startSshSession(login, node.id);
  };

  return (
    <MenuLogin
      width="90px"
      textTransform="none"
      alignButtonWidthToMenu
      getLoginItems={handleOnOpen}
      onSelect={handleOnSelect}
      transformOrigin={{
        vertical: 'top',
        horizontal: 'right',
      }}
      anchorOrigin={{
        vertical: 'center',
        horizontal: 'right',
      }}
    />
  );
};

const DesktopConnect = ({ desktop }: { desktop: Desktop }) => {
  const { clusterId } = useStickyClusterId();
  const startRemoteDesktopSession = (username: string, desktopName: string) => {
    const url = cfg.getDesktopRoute({
      clusterId,
      desktopName,
      username,
    });

    openNewTab(url);
  };

  function handleOnOpen() {
    return makeDesktopLoginOptions(clusterId, desktop.name, desktop.logins);
  }

  function handleOnSelect(e: React.SyntheticEvent, login: string) {
    e.preventDefault();
    return startRemoteDesktopSession(login, desktop.name);
  }

  return (
    <MenuLogin
      width="90px"
      textTransform="none"
      alignButtonWidthToMenu
      getLoginItems={handleOnOpen}
      onSelect={handleOnSelect}
      transformOrigin={{
        vertical: 'top',
        horizontal: 'right',
      }}
      anchorOrigin={{
        vertical: 'center',
        horizontal: 'right',
      }}
    />
  );
};

const AppLaunch = ({ app }: { app: App }) => {
  const {
    launchUrl,
    awsConsole,
    awsRoles,
    fqdn,
    clusterId,
    publicAddr,
    isCloudOrTcpEndpoint,
    samlApp,
    samlAppSsoUrl,
  } = app;
  if (awsConsole) {
    return (
      <AwsLaunchButton
        awsRoles={awsRoles}
        fqdn={fqdn}
        clusterId={clusterId}
        publicAddr={publicAddr}
      />
    );
  }
  if (isCloudOrTcpEndpoint) {
    return (
      <ButtonBorder
        disabled
        width="90px"
        size="small"
        title="Cloud or TCP applications cannot be launched by the browser"
        textTransform="none"
      >
        Launch
      </ButtonBorder>
    );
  }
  if (samlApp) {
    return (
      <ButtonBorder
        as="a"
        width="90px"
        size="small"
        target="_blank"
        href={samlAppSsoUrl}
        rel="noreferrer"
        textTransform="none"
      >
        Login
      </ButtonBorder>
    );
  }
  return (
    <ButtonBorder
      as="a"
      width="90px"
      size="small"
      target="_blank"
      href={launchUrl}
      rel="noreferrer"
      textTransform="none"
    >
      Launch
    </ButtonBorder>
  );
};

function DatabaseConnect({ database }: { database: Database }) {
  const { name, protocol } = database;
  const ctx = useTeleport();
  const { clusterId } = useStickyClusterId();
  const [open, setOpen] = useState(false);
  const username = ctx.storeUser.state.username;
  const authType = ctx.storeUser.state.authType;
  const accessRequestId = ctx.storeUser.getAccessRequestId();
  return (
    <>
      <ButtonBorder
        textTransform="none"
        width="90px"
        size="small"
        onClick={() => {
          setOpen(true);
        }}
      >
        Connect
      </ButtonBorder>
      {open && (
        <DbConnectDialog
          username={username}
          clusterId={clusterId}
          dbName={name}
          dbProtocol={protocol}
          onClose={() => setOpen(false)}
          authType={authType}
          accessRequestId={accessRequestId}
        />
      )}
    </>
  );
}

const KubeConnect = ({ kube }: { kube: Kube }) => {
  const ctx = useTeleport();
  const { clusterId } = useStickyClusterId();
  const [open, setOpen] = useState(false);
  const username = ctx.storeUser.state.username;
  const authType = ctx.storeUser.state.authType;
  const accessRequestId = ctx.storeUser.getAccessRequestId();
  return (
    <>
      <ButtonBorder
        width="90px"
        textTransform="none"
        size="small"
        onClick={() => setOpen(true)}
      >
        Connect
      </ButtonBorder>
      {open && (
        <KubeConnectDialog
          onClose={() => setOpen(false)}
          username={username}
          authType={authType}
          kubeConnectName={kube.name}
          clusterId={clusterId}
          accessRequestId={accessRequestId}
        />
      )}
    </>
  );
};

const sortNodeLogins = (logins: string[]) => {
  const noRoot = logins.filter(l => l !== 'root').sort();
  if (noRoot.length === logins.length) {
    return logins;
  }
  return ['root', ...noRoot];
};

const makeNodeOptions = (clusterId: string, node: Node | undefined) => {
  const nodeLogins = node?.sshLogins || [];
  const logins = sortNodeLogins(nodeLogins);

  return logins.map(login => {
    const url = cfg.getSshConnectRoute({
      clusterId,
      serverId: node?.id || '',
      login,
    });

    return {
      login,
      url,
    };
  });
};

const makeDesktopLoginOptions = (
  clusterId: string,
  desktopName = '',
  logins = [] as string[]
): LoginItem[] => {
  return logins.map(username => {
    const url = cfg.getDesktopRoute({
      clusterId,
      desktopName,
      username,
    });

    return {
      login: username,
      url,
    };
  });
};
