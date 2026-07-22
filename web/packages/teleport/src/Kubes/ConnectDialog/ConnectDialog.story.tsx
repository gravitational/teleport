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

import Component from './ConnectDialog';

export default {
  title: 'Teleport/Kubes/Connect',
};

export const Local = () => {
  return (
    <Component
      onClose={() => null}
      username={'sam'}
      authType={'local'}
      kubeConnectName={'tele.logicoma.dev-prod'}
      clusterId={'some-cluster-name'}
    />
  );
};

export const LocalWithRequestId = () => {
  return (
    <Component
      onClose={() => null}
      username={'sam'}
      authType={'local'}
      kubeConnectName={'tele.logicoma.dev-prod'}
      clusterId={'some-cluster-name'}
      accessRequestId={'8289cdb1-385c-5b02-85f1-fa2a934b749f'}
    />
  );
};

export const Sso = () => {
  return (
    <Component
      onClose={() => null}
      username={'sam'}
      authType={'sso'}
      kubeConnectName={'tele.logicoma.dev-prod'}
      clusterId={'some-cluster-name'}
    />
  );
};
