import { useEffect, useState } from 'react';
import { matchPath, useHistory, useLocation } from 'react-router';

import { routePaths } from '../../config/routes';
import { HistoryService } from '../../services/history/historyService';
import { SessionService } from '../../services/session/sessionService';

function getRedirectUri(search: string) {
  const params = new URLSearchParams(search);

  let entryUrl = params.get('redirect_url');

  if (entryUrl) {
    entryUrl = HistoryService.ensureKnownRoute(entryUrl);
  } else {
    entryUrl = routePaths.root;
  }

  return HistoryService.ensureBaseUrl(entryUrl);
}

export function useCheckSessionAndRedirect() {
  const [checkingValidSession, setCheckingValidSession] = useState(
    SessionService.isValid()
  );

  const history = useHistory();

  const { search } = useLocation();

  useEffect(() => {
    if (!SessionService.isValid()) {
      return;
    }

    try {
      const redirectUrlWithBase = new URL(getRedirectUri(search));

      const matched = matchPath(redirectUrlWithBase.pathname, {
        path: routePaths.samlIdpSso,
        strict: true,
        exact: true,
      });

      if (matched) {
        HistoryService.refreshPage(redirectUrlWithBase.toString());

        return;
      }

      history.replace(routePaths.root);
    } catch (e) {
      console.error(e);

      history.replace(routePaths.root);
      return;
    }

    setCheckingValidSession(false);
  }, [history, search]);

  return checkingValidSession;
}
