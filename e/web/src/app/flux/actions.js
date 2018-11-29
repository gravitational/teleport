import $ from 'jQuery';

// telebase imports
import { fetchInitData } from 'telebase-app/flux/app/actions';
import { initAppStatus } from 'telebase-app/flux/status/actions';

// local
import * as backend from '../services/backend';
import { fetchLicenseStatus } from './license/actions';

export function initApp(siteId, featureActivator) {
  initAppStatus.start();
  return $.when(fetchInitData(siteId), fetchLicenseStatus())
    .done(() => {
      featureActivator.onload();
      initAppStatus.success();
    })
    .fail(err => {
      const msg = backend.getErrorText(err);
      initAppStatus.fail(msg);
    })
}

