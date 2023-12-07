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

import React, { useCallback, useEffect } from 'react';

import { useLocation, useParams } from 'react-router';

import { Flex, Indicator } from 'design';

import { AccessDenied } from 'design/CardError';

import useAttempt from 'shared/hooks/useAttemptNext';

import { UrlLauncherParams } from 'teleport/config';
import service from 'teleport/services/apps';

export function AppLauncher() {
  const { attempt, setAttempt } = useAttempt('processing');

  const params = useParams<UrlLauncherParams>();
  const { search } = useLocation();
  const queryParams = new URLSearchParams(search);

  const createAppSession = useCallback(async (params: UrlLauncherParams) => {
    let fqdn = params.fqdn;
    const port = location.port ? `:${location.port}` : '';

    try {
      if (!fqdn) {
        const app = await service.getAppFqdn(params);
        fqdn = app.fqdn;
      }

      // Decode URL encoded values from the ARN.
      if (params.arn) {
        params.arn = decodeURIComponent(params.arn);
      }

      const session = await service.createAppSession(params);

      // Setting cookie
      await fetch(`https://${fqdn}${port}/x-teleport-auth`, {
        method: 'POST',
        credentials: 'include',
        headers: {
          'X-Cookie-Value': session.cookieValue,
          'X-Subject-Cookie-Value': session.subjectCookieValue,
        },
      });

      let path = '';
      if (queryParams.has('path')) {
        path = queryParams.get('path');

        if (path && !path.startsWith('/')) {
          path = `/${path}`;
        }

        if (queryParams.has('query')) {
          path += '?' + queryParams.get('query');
        }
      }

      window.location.replace(`https://${fqdn}${port}${path}`);
    } catch (err) {
      let statusText = 'Something went wrong';

      if (err instanceof TypeError) {
        // `fetch` returns `TypeError` when there is a network error.
        statusText = `Unable to access "${fqdn}". This may happen if your Teleport Proxy is using untrusted or self-signed certificate. Please ensure Teleport Proxy service uses valid certificate or access the application domain directly (https://${fqdn}${port}) and accept the certificate exception from your browser.`;
      } else if (err instanceof Error) {
        statusText = err.message;
      }

      setAttempt({
        status: 'failed',
        statusText,
      });
    }
  }, []);

  useEffect(() => {
    createAppSession(params);
  }, [params]);

  if (attempt.status === 'failed') {
    return <AppLauncherAccessDenied statusText={attempt.statusText} />;
  }

  return <AppLauncherProcessing />;
}

export function AppLauncherProcessing() {
  return (
    <Flex height="180px" justifyContent="center" alignItems="center" flex="1">
      <Indicator />
    </Flex>
  );
}

interface AppLauncherAccessDeniedProps {
  statusText: string;
}

export function AppLauncherAccessDenied(props: AppLauncherAccessDeniedProps) {
  return <AccessDenied message={props.statusText} />;
}
