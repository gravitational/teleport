/*
Copyright 2022 Gravitational, Inc.

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
import Logger from 'shared/libs/logger';

import { alertNames, fetchClusterAlerts } from 'teleport/services/alerts';
import useStickyClusterId from 'teleport/useStickyClusterId';

import cfg from 'teleport/config';

import type { ClusterAlert } from 'teleport/services/alerts';

const logger = Logger.create('ClusterAlerts');

const DISABLED_ALERTS = 'disabledAlerts';
const MS_HOUR = 60 * 60 * 1000;

export function addHours(date: number, hours: number) {
  return date + hours * MS_HOUR;
}

function getItem(key: string): string | null {
  return window.localStorage.getItem(key);
}

function setItem(key: string, data: string) {
  window.localStorage.setItem(key, data);
}

type DismissedAlerts = {
  [alertName: string]: number;
};

export function useAlerts(initialAlerts: ClusterAlert[] = []) {
  const [alerts, setAlerts] = useState<ClusterAlert[]>(initialAlerts);
  const [dismissedAlerts, setDismissedAlerts] = useState<DismissedAlerts>({});
  const { clusterId } = useStickyClusterId();

  useEffect(() => {
    const disabledAlerts = getItem(DISABLED_ALERTS);
    if (disabledAlerts) {
      // Loop through the existing ones and remove those that have passed 24h.
      const data: DismissedAlerts = JSON.parse(disabledAlerts);
      Object.entries(data).forEach(([name, expiry]) => {
        if (new Date().getTime() > +expiry) {
          delete data[name];
        }
      });
      setDismissedAlerts(data);
      setItem(DISABLED_ALERTS, JSON.stringify(data));
    }
  }, []);

  useEffect(() => {
    fetchClusterAlerts(clusterId)
      .then(res => {
        if (!res) {
          return;
        }

        // filter upgrade suggestions from showing on dashboards
        if (cfg.isDashboard) {
          res = res.filter(alert => {
            return (
              alert.metadata.name !== alertNames.RELEASE_ALERT_ID &&
              alert.metadata.name !== alertNames.SEC_ALERT_ID
            );
          });
        }

        setAlerts(res);
      })
      .catch(err => {
        logger.error(err);
      });
  }, [clusterId]);

  function dismissAlert(name: string) {
    const disabledAlerts = getItem(DISABLED_ALERTS);
    let data: DismissedAlerts = {};
    if (disabledAlerts) {
      data = JSON.parse(disabledAlerts);
    }
    data[name] = addHours(new Date().getTime(), 24);
    setDismissedAlerts(data);
    setItem(DISABLED_ALERTS, JSON.stringify(data));
  }

  const dismissedAlertNames = Object.keys(dismissedAlerts);

  const visibleAlerts = alerts.filter(
    alert => !dismissedAlertNames.includes(alert.metadata.name)
  );

  return {
    alerts: visibleAlerts,
    dismissAlert,
  };
}
