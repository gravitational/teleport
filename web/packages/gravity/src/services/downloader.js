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

import history from './history';
import localStorage from './localStorage';

export function makeDownloadable(url){
  const accessToken = localStorage.getAccessToken();
  const downloadUrl = `${url}?access_token=${accessToken}`;
  return history.ensureBaseUrl(downloadUrl);
}

export function download(url){
  const downloadUrl = makeDownloadable(url);
  const element = document.createElement('a');
  element.setAttribute('href', `${downloadUrl}`);
  // works in ie11
  element.setAttribute('target', `_blank`);
  element.style.display = 'none';
  document.body.appendChild(element);
  element.click();
  document.body.removeChild(element);
}