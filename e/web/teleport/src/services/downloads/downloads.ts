import api from 'teleport/services/api';

import cfg from 'e-teleport/config';

import { makeReleases } from './make';

export const downloadsService = {
  fetchReleases() {
    return api.get(cfg.api.releases).then(makeReleases);
  },
  fetchLicense(): Promise<string> {
    return api.get(cfg.api.license);
  },
};

export const downloadObject = (filename: string, text: string) => {
  /*
   * http://stackoverflow.com/questions/3665115/create-a-file-in-memory-for-user-to-download-not-through-server
   */
  var element = document.createElement('a');
  element.setAttribute(
    'href',
    'data:text/plain;charset=utf-8,' + encodeURIComponent(text)
  );
  element.setAttribute('download', filename);
  element.style.display = 'none';

  document.body.appendChild(element);

  element.click();
  document.body.removeChild(element);
};
