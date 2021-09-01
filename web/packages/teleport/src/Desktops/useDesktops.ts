/*
Copyright 2021 Gravitational, Inc.

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

import { useState, useEffect } from 'react';
import useAttempt from 'shared/hooks/useAttemptNext';
import Ctx from 'teleport/teleportContext';
import useStickyClusterId from 'teleport/useStickyClusterId';
import { Desktop } from 'teleport/services/desktops';
import cfg from 'teleport/config';

export default function useDesktops(ctx: Ctx) {
  const { attempt, run } = useAttempt('processing');
  const { clusterId, isLeafCluster } = useStickyClusterId();
  const username = ctx.storeUser.state.username;
  const canCreate = ctx.storeUser.getTokenAccess().create;
  const isEnterprise = ctx.isEnterprise;
  const version = ctx.storeUser.state.cluster.authVersion;
  const authType = ctx.storeUser.state.authType;

  const [searchValue, setSearchValue] = useState<string>('');

  const [desktops, setDesktops] = useState<Desktop[]>([]);

  useEffect(() => {
    run(() => ctx.desktopService.fetchDesktops(clusterId).then(setDesktops));
  }, [clusterId]);

  const openRemoteDesktopWindow = (username: string, desktopId: string) => {
    const url = cfg.getDesktopRoute({
      clusterId,
      desktopId,
      username,
    });

    openNewWindow(url);
  };

  // It turns out opening a new window is not as simple as one would hope with modern browsers.
  // The following solution works on recent versions of Chrome/Firefox/Safari, but it may be unstable
  // since it is not explicitly defined behavior. For now it should be tested manually before releases.
  const openNewWindow = (url: string) => {
    const element = document.createElement('a');
    // see https://forums.asp.net/post/4841258.aspx
    element.setAttribute('href', `${url}`);
    element.setAttribute(
      'onclick',
      `window.open(this.href, '_blank', 'scrollbars=no,status=no,toolbar=no,menubar=no,location=no'); return false;`
    );
    element.style.display = 'none';
    document.body.appendChild(element);
    element.click();
    document.body.removeChild(element);
  };

  return {
    desktops,
    attempt,
    canCreate,
    isLeafCluster,
    isEnterprise,
    username,
    version,
    clusterId,
    authType,
    searchValue,
    setSearchValue,
    openRemoteDesktopWindow,
  };
}

export type State = ReturnType<typeof useDesktops>;
