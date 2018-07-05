import api from './api';
import { EventEmitter } from 'events';

class Transfer extends EventEmitter {

  constructor(){
    super();
    this._xhr = new XMLHttpRequest();
  }

  abort(){
    this._xhr.abort();
  }

  onProgress(cb) {
    this.on('progress', cb);
  }

  onCompleted(cb) {
    this.on('completed', cb);
  }

  onError(cb) {
    this.on('error', cb);
  }

  handleProgress(e) {
    let progress = 0;
    // if Content-Length is present
    if (e.lengthComputable) {
      progress = Math.round((e.loaded/e.total)*100);
    } else {
      const done = e.position || e.loaded;
      const total = e.totalSize || e.total;
      progress = Math.floor(done / total * 1000) / 10;
    }

    this.emit('progress', progress);
  }

}

export class Uploader extends Transfer {
  constructor(){
    super();
  }

  do(url, blob){
    this._xhr.upload.addEventListener('progress',
      e => this.handleProgress(e))

    const xhr = this._xhr;
    return api.ajax({
      url: url,
      type: 'POST',
      data: blob,
      cache : false,
      processData: false,
      contentType: false,
      xhr() {
        return xhr;
      }
    })
    .done(json => {
      this.emit('completed', json);
    })
    .error(err =>{
      const msg = api.getErrorText(err);
      this.emit('error', new Error(msg));
    })
  }
}

export class Downloader extends Transfer {
  constructor(){
    super();
  }

  do(url) {
    const xhr = this._xhr;
    xhr.onprogress = e => this.handleProgress(e)
    xhr.onload = () => {
      const { status, response } = xhr;
      if (status === 200) {
        this._handleSuccess(xhr)
        return;
      }

      this._handleError(response)
    }

    xhr.open('get', url, true);
    xhr.responseType = 'blob';

    api.setAuthHeaders(xhr)
    xhr.send();
  }

  _handleSuccess(xhr) {
    const fileName = getDispositionFileName(xhr);
    if (!fileName) {
      this.emit('error', new Error("Bad response"));
    } else {
      this.emit('completed', {
        fileName: fileName,
        blob: xhr.response
      });
    }
  }

  _handleError(response) {
    const defaultMessage = 'Bad request';
    if (!window.FileReader) {
      this.emit('error', new Error(defaultMessage));
      return;
    }

    const reader = new FileReader(response);
    reader.onerror = () => this.emit('error', Error(defaultMessage));
    reader.onload = () => {
      try {
        const { message } = JSON.parse(reader.result);
        this.emit('error', new Error(message));
      } catch (err) {
        this.emit('error', new Error("Bad response"));
      }
    }

    reader.readAsText(response);
  }
}

function getDispositionFileName(xhr) {
  let fileName = "";
  const disposition = xhr.getResponseHeader("Content-Disposition");
  if (disposition) {
    var filenameRegex = /filename[^;=\n]*=((['"]).*?\2|[^;\n]*)/;
    var matches = filenameRegex.exec(disposition);
    if (matches != null && matches[1]) {
      fileName = matches[1].replace(/['"]/g, '');
    }
  }

  return decodeURIComponent(fileName);
}