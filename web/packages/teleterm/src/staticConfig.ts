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

/**
 * `StaticConfig` allows providing different values between the dev build and
 * the packaged app.
 * The proper config is resolved by Vite at compile time.
 * This differs from `RuntimeSettings`, where properties are resolved during
 * the app's runtime.
 */

interface IStaticConfig {
  prehogAddress: string;
  feedbackAddress: string;
}

let staticConfig: IStaticConfig;

if (process.env.NODE_ENV === 'production') {
  staticConfig = {
    prehogAddress: 'https://reporting-connect.teleportinfra.sh',
    feedbackAddress: 'https://usage.teleport.dev',
  };
} else {
  staticConfig = {
    prehogAddress: 'https://reporting-connect-staging.teleportinfra.dev',
    feedbackAddress:
      'https://kcwm2is93l.execute-api.us-west-2.amazonaws.com/prod',
  };
}

export { staticConfig };
