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

/*
Copyright (c) 2012 Tai-Jin Lee http://www.taijinlee.com
Permission is hereby granted, free of charge, to any person obtaining
a copy of this software and associated documentation files (the
"Software"), to deal in the Software without restriction, including
without limitation the rights to use, copy, modify, merge, publish,
distribute, sublicense, and/or sell copies of the Software, and to
permit persons to whom the Software is furnished to do so, subject to
the following conditions:

The above copyright notice and this permission notice shall be
included in all copies or substantial portions of the Software.

THE SOFTWARE IS PROVIDED "AS IS", WITHOUT WARRANTY OF ANY KIND,
EXPRESS OR IMPLIED, INCLUDING BUT NOT LIMITED TO THE WARRANTIES OF
MERCHANTABILITY, FITNESS FOR A PARTICULAR PURPOSE AND
NONINFRINGEMENT. IN NO EVENT SHALL THE AUTHORS OR COPYRIGHT HOLDERS BE
LIABLE FOR ANY CLAIM, DAMAGES OR OTHER LIABILITY, WHETHER IN AN ACTION
OF CONTRACT, TORT OR OTHERWISE, ARISING FROM, OUT OF OR IN CONNECTION
WITH THE SOFTWARE OR THE USE OR OTHER DEALINGS IN THE SOFTWARE.
*/

/**
 * format number by adding thousands separaters and significant digits while rounding
 */
function numberFormat(number, decimals, decPoint, thousandsSep) {
  decimals = isNaN(decimals) ? 2 : Math.abs(decimals);
  decPoint = (decPoint === undefined) ? '.' : decPoint;
  thousandsSep = (thousandsSep === undefined) ? ',' : thousandsSep;

  var sign = number < 0 ? '-' : '';
  number = Math.abs(+number || 0);

  var intPart = parseInt(number.toFixed(decimals), 10) + '';
  var j = intPart.length > 3 ? intPart.length % 3 : 0;

  return sign + (j ? intPart.substr(0, j) + thousandsSep : '') + intPart.substr(j).replace(/(\d{3})(?=\d)/g, '$1' + thousandsSep) + (decimals ? decPoint + Math.abs(number - intPart).toFixed(decimals).slice(2) : '');
}

/**
 * Formats the value like a 'human-readable' file size (i.e. '13 KB', '4.1 MB', '102 bytes', etc).
 *
 * For example:
 * If value is 123456789, the output would be 117.7 MB.
 */
export function filesize(filesize, kilo, decimals, decPoint, thousandsSep, suffixSep) {
  kilo = (kilo === undefined) ? 1024 : kilo;
  if (filesize <= 0) { return '0 bytes'; }
  if (filesize < kilo && decimals === undefined) { decimals = 0; }
  if (suffixSep === undefined) { suffixSep = ' '; }
  return intword(filesize, ['bytes', 'KB', 'MB', 'GB', 'TB', 'PB'], kilo, decimals, decPoint, thousandsSep, suffixSep);
}

/**
 * Formats the value like a 'human-readable' number (i.e. '13 K', '4.1 M', '102', etc).
 *
 * For example:
 * If value is 123456789, the output would be 117.7 M.
 */
export function intword(number, units, kilo, decimals, decPoint, thousandsSep, suffixSep) {
  var humanized, unit;

  units = units || ['', 'K', 'M', 'B', 'T'],
  unit = units.length - 1,
  kilo = kilo || 1000,
  decimals = isNaN(decimals) ? 2 : Math.abs(decimals),
  decPoint = decPoint || '.',
  thousandsSep = thousandsSep || ',',
  suffixSep = suffixSep || '';

  for (var i=0; i < units.length; i++) {
    if (number < Math.pow(kilo, i+1)) {
      unit = i;
      break;
    }
  }
  humanized = number / Math.pow(kilo, unit);

  var suffix = units[unit] ? suffixSep + units[unit] : '';
  return numberFormat(humanized, decimals, decPoint, thousandsSep) + suffix;
}