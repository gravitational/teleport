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

/**
 * `StaticConfig` allows providing different values between the dev build and
 * the packaged app.
 * The proper config is resolved by webpack at compile time.
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
