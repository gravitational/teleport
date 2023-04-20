/*
Copyright 2023 Gravitational, Inc.

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

export function getStyledComponentsConfig(mode: string) {
  // https://nextjs.org/docs/advanced-features/compiler#styled-components
  if (mode === 'production') {
    return {
      ssr: false,
      pure: false, // not currently supported by SWC
      displayName: false,
      fileName: false,
      cssProp: true,
    };
  }

  return {
    ssr: false,
    pure: true, // not currently supported by SWC
    displayName: true,
    fileName: true,
    cssProp: true,
  };
}
