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

import { useEffect, useState } from 'react';

import Logger from 'shared/libs/logger';

import cfg from 'teleport/config';
import {
  alertNames,
  fetchClusterAlerts,
  type ClusterAlert,
} from 'teleport/services/alerts';
import useStickyClusterId from 'teleport/useStickyClusterId';

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
