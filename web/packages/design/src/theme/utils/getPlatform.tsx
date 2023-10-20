/*
Copyright 2019 Gravitational, Inc.

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

export enum Platform {
  PLATFORM_WINDOWS = 'Windows',
  PLATFORM_MACINTOSH = 'Macintosh',
  PLATFORM_LINUX = 'Linux',
  PLATFORM_WIN32 = 'win32',
  PLATFORM_DARWIN = 'darwin',
}

type PlatformType = {
  isWin: boolean;
  isMac: boolean;
  isLinux: boolean;
};

export default function getPlatform(): PlatformType {
  if (typeof window !== 'undefined') {
    const userAgent = window.navigator.userAgent;

    return {
      isWin: userAgent.indexOf(Platform.PLATFORM_WINDOWS) >= 0,
      isMac: userAgent.indexOf(Platform.PLATFORM_MACINTOSH) >= 0,
      isLinux: userAgent.indexOf(Platform.PLATFORM_LINUX) >= 0,
    };
  } else {
    const platform = process.platform;
    const isWin = platform === Platform.PLATFORM_WIN32;
    const isMac = platform === Platform.PLATFORM_DARWIN;

    return {
      isWin,
      isMac,
      isLinux: !isWin && !isMac,
    };
  }
}
