/* The MIT License (MIT)

Copyright (c) 2016 Jordan Harband

Permission is hereby granted, free of charge, to any person obtaining a copy
of this software and associated documentation files (the "Software"), to deal
in the Software without restriction, including without limitation the rights
to use, copy, modify, merge, publish, distribute, sublicense, and/or sell
copies of the Software, and to permit persons to whom the Software is
furnished to do so, subject to the following conditions:

The above copyright notice and this permission notice shall be included in all
copies or substantial portions of the Software.

THE SOFTWARE IS PROVIDED "AS IS", WITHOUT WARRANTY OF ANY KIND, EXPRESS OR
IMPLIED, INCLUDING BUT NOT LIMITED TO THE WARRANTIES OF MERCHANTABILITY,
FITNESS FOR A PARTICULAR PURPOSE AND NONINFRINGEMENT. IN NO EVENT SHALL THE
AUTHORS OR COPYRIGHT HOLDERS BE LIABLE FOR ANY CLAIM, DAMAGES OR OTHER
LIABILITY, WHETHER IN AN ACTION OF CONTRACT, TORT OR OTHERWISE, ARISING FROM,
OUT OF OR IN CONNECTION WITH THE SOFTWARE OR THE USE OR OTHER DEALINGS IN THE
SOFTWARE. */

if (typeof Promise !== 'function') {
  throw new TypeError('A global Promise is required');
}

if (typeof Promise.prototype.finally !== 'function') {
  var speciesConstructor = function (O, defaultConstructor) {
    if (!O || (typeof O !== 'object' && typeof O !== 'function')) {
      throw new TypeError('Assertion failed: Type(O) is not Object');
    }
    var C = O.constructor;
    if (typeof C === 'undefined') {
      return defaultConstructor;
    }
    if (!C || (typeof C !== 'object' && typeof C !== 'function')) {
      throw new TypeError('O.constructor is not an Object');
    }
    var S =
      typeof Symbol === 'function' && typeof Symbol.species === 'symbol'
        ? C[Symbol.species]
        : undefined;
    if (S == null) {
      return defaultConstructor;
    }
    if (typeof S === 'function' && S.prototype) {
      return S;
    }
    throw new TypeError('no constructor found');
  };

  var shim = {
    finally(onFinally) {
      var promise = this;
      if (typeof promise !== 'object' || promise === null) {
        throw new TypeError('"this" value is not an Object');
      }
      var C = speciesConstructor(promise, Promise); // throws if SpeciesConstructor throws
      if (typeof onFinally !== 'function') {
        return Promise.prototype.then.call(promise, onFinally, onFinally);
      }
      return Promise.prototype.then.call(
        promise,
        x => new C(resolve => resolve(onFinally())).then(() => x),
        e =>
          new C(resolve => resolve(onFinally())).then(() => {
            throw e;
          })
      );
    },
  };

  Object.defineProperty(Promise.prototype, 'finally', {
    configurable: true,
    writable: true,
    value: shim.finally,
  });
}
