/**
 * Teleport
 * Copyright (C) 2023  Gravitational, Inc.
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

import React, { useState } from 'react';
import { ButtonBorder } from 'design';
import { LoginItem, MenuLogin } from 'shared/components/MenuLogin';
import { AwsLaunchButton } from 'shared/components/AwsLaunchButton';

import { UnifiedResource } from 'teleport/services/agents';
import cfg from 'teleport/config';

import useTeleport from 'teleport/useTeleport';
import { Database } from 'teleport/services/databases';
import { openNewTab } from 'teleport/lib/util';
import { Kube } from 'teleport/services/kube';
import { Desktop } from 'teleport/services/desktops';
import DbConnectDialog from 'teleport/Databases/ConnectDialog';
import KubeConnectDialog from 'teleport/Kubes/ConnectDialog';
import useStickyClusterId from 'teleport/useStickyClusterId';
import { Node, sortNodeLogins } from 'teleport/services/nodes';
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
      textTransform={'none'}
      alignButtonWidthToMenu
      getLoginItems={handleOnOpen}
      onSelect={handleOnSelect}
      transformOrigin={{
        vertical: 'top',
        horizontal: 'right',
      }}
      anchorOrigin={{
        vertical: 'bottom',
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
        vertical: 'bottom',
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
        getLaunchUrl={arn =>
          cfg.getAppLauncherRoute({
            fqdn,
            clusterId,
            publicAddr,
            arn,
          })
        }
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
