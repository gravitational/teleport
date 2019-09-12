//
// taken and modified from https://github.com/mo22/express-modify-response
//
module.exports = function expressModifyResponse(checkCallback, modifyCallback) {
  return function expressModifyResponse(req, res, next) {
    var _end = res.end;
    var _write = res.write;
    var checked = false;
    var buffers = [];
    var addBuffer = (chunk, encoding) => {
      if (chunk === undefined) 
        return;
      if (typeof chunk === 'string') {
        chunk = new Buffer(chunk, encoding);
      }
      buffers.push(chunk);
    };

    var _writeHead = res.writeHead;

    res.writeHead = function () {
      // writeHead supports (statusCode, headers) as well as (statusCode,
      // statusMessage, headers)
      var headers = (arguments.length > 2)
        ? arguments[2]
        : arguments[1];
      var contentType = this.getHeader('content-type');

      if ((typeof contentType != 'undefined') && (contentType.indexOf('text/html') == 0)) {
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

      Promise
        .resolve(modifyCallback(req, res, buffer))
        .then((result) => {
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
        .catch((e) => {
          // handle?
          next(e);
        });
    };

    next();
  }
}