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

const htmlUtils = {

  copyToClipboard(textToCopy){
    let aux = document.createElement('textarea');
    aux.value = textToCopy;
    document.body.appendChild(aux);
    aux.select();
    document.execCommand("copy");
    document.body.removeChild(aux);
  },

  selectElementContent(element){
    var range, selection;
    if (window.getSelection && document.createRange) {
      selection = window.getSelection();
      range = document.createRange();
      range.selectNodeContents(element);
      selection.removeAllRanges();
      selection.addRange(range);
    } else if (document.selection && document.body.createTextRange) {
      range = document.body.createTextRange();
      range.moveToElementText(element);
      range.select();
    }
  },

  download(filename, text) {
    /*
    * http://stackoverflow.com/questions/3665115/create-a-file-in-memory-for-user-to-download-not-through-server
    */
    var element = document.createElement('a');
    element.setAttribute('href', 'data:text/plain;charset=utf-8,' + encodeURIComponent(text));
    element.setAttribute('download', filename);
    element.style.display = 'none';

    document.body.appendChild(element);

    element.click();
    document.body.removeChild(element);
  },

  joinPaths(path1, path2) {
    return `${path1}/${path2}`.replace(/\/\/+/g, '/');
  }
}

export default htmlUtils
