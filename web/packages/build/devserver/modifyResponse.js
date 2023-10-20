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

// This handler reads bearer and CSTF tokens from original index.html
// and inserts these values into the local build version.
// This allows authentication against targeted server.
module.exports = function modifyIndexHtmlMiddleware(compiler) {
  return modifyResponse(
    (req, res) => {
      // return true if you want to modify the response later
      const contentType = res.getHeader('Content-Type');
      if (contentType && contentType.startsWith('text/html')) {
        return true;
      }

      return false;
    },
    (req, res, body) => {
      // clear SCP headers because Gravitational SCP headers
      // prevents inline JS execution required by hot-reloads to work
      res.set({
        'content-security-policy': '',
      });

      // bodyText is the text of the server response
      const bodyText = body.toString();

      let html = compiler.readLocalIndexHtml();
      html = injectCsrf(bodyText, html);
      html = injectBearer(bodyText, html);
      return html;
    }
  );
};

function injectCsrf(source, target) {
  var value = source.match(
    new RegExp(/<meta name="grv_csrf_token" content="[a-zA-Z0-9_.-=]*"( )?\/>/)
  );
  if (value) {
    return target.replace(
      new RegExp(/<meta name="grv_csrf_token" content="{{ \.XCSRF }}"( )?\/>/),
      value[0]
    );
  }

  return target;
}

function injectBearer(source, target) {
  var value = source.match(
    new RegExp(
      /<meta name="grv_bearer_token" content="[a-zA-Z0-9_.-=]*"( )?\/>/
    )
  );
  if (value) {
    return target.replace(
      new RegExp(
        /<meta name="grv_bearer_token" content="{{ \.Session }}"( )?\/>/
      ),
      value[0]
    );
  }
  return target;
}

//
// taken and modified from https://github.com/mo22/express-modify-response
//
function modifyResponse(checkCallback, modifyCallback) {
  return function expressModifyResponse(req, res, next) {
    var _end = res.end;
    var _write = res.write;
    var checked = false;
    var buffers = [];
    var addBuffer = (chunk, encoding) => {
      if (chunk === undefined) return;
      if (typeof chunk === 'string') {
        chunk = new Buffer(chunk, encoding);
      }
      buffers.push(chunk);
    };

    var _writeHead = res.writeHead;

    res.writeHead = function () {
      // writeHead supports (statusCode, headers) as well as (statusCode,
      // statusMessage, headers)
      var headers = arguments.length > 2 ? arguments[2] : arguments[1];
      var contentType = this.getHeader('content-type');

      if (
        typeof contentType != 'undefined' &&
        contentType.indexOf('text/html') == 0
      ) {
        res.isHtml = true;

        // Strip off the content length since it will change.
        res.removeHeader('Content-Length');

        if (headers) {
          delete headers['content-length'];
        }
      }

      _writeHead.apply(res, arguments);
    };

    res.write = function write(chunk, encoding) {
      if (!checked) {
        checked = true;
        var hook = checkCallback(req, res);
        if (!hook) {
          res.end = _end;
          res.write = _write;
          return res.write(chunk, encoding);
        } else {
          addBuffer(chunk, encoding);
        }
      } else {
        addBuffer(chunk, encoding);
      }
    };

    res.end = function end(chunk, encoding) {
      if (!checked) {
        checked = true;
        var hook = checkCallback(req, res);
        if (!hook) {
          res.end = _end;
          res.write = _write;
          return res.end(chunk, encoding);
        } else {
          addBuffer(chunk, encoding);
        }
      } else {
        addBuffer(chunk, encoding);
      }
      var buffer = Buffer.concat(buffers);

      Promise.resolve(modifyCallback(req, res, buffer))
        .then(result => {
          if (typeof result === 'string') {
            result = new Buffer(result, 'utf-8');
          }

          if (res.getHeader('Content-Length')) {
            res.setHeader('Content-Length', String(result.length));
          }
          res.end = _end;
          res.write = _write;
          res.write(result);
          res.end();
        })
        .catch(e => {
          // handle?
          next(e);
        });
    };

    next();
  };
}
