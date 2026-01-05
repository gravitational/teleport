/**
 * Teleport
 * Copyright (C) 2026 Gravitational, Inc.
 *
 * This program is free software: you can redistribute it and/or modify
 * it under the terms of the GNU Affero General Public License as published by
 * the Free Software Foundation, either version 3 of the License, or
 * (at your option) any later version.
 *
 * This program is distributed in the hope that it will be useful,
 * but WITHOUT ANY WARRANTY; without even the implied warranty of
 * MERCHANTABILITY or FITNESS FOR A PARTICULAR PURPOSE.  See the
 * GNU Affero General Public License for more details.
 *
 * You should have received a copy of the GNU Affero General Public License
 * along with this program.  If not, see <http://www.gnu.org/licenses/>.
 */

export function parseAddress(address: string): { host: string; port?: number } {
  if (!address) {
    return { host: address };
  }

  if (address.startsWith('[')) {
    const endBracket = address.indexOf(']');
    if (endBracket !== -1) {
      const host = address.substring(1, endBracket);
      const remainder = address.substring(endBracket + 1);
      
      if (remainder.startsWith(':')) {
        const portStr = remainder.substring(1);
        const port = parseInt(portStr, 10);
        if (!isNaN(port)) {
          return { host, port };
        }
      }
      
      return { host };
    }
  }

  const colonCount = (address.match(/:/g) || []).length;
  
  if (colonCount > 1) {
    return { host: address };
  }

  if (colonCount === 1) {
    const lastColon = address.lastIndexOf(':');
    const portStr = address.substring(lastColon + 1);
    const port = parseInt(portStr, 10);
    
    if (!isNaN(port) && port > 0 && port <= 65535) {
      return {
        host: address.substring(0, lastColon),
        port
      };
    }
  }

  return { host: address };
}

export function extractHost(address: string): string {
  return parseAddress(address).host;
}

export function isLocalhost(address: string): boolean {
  if (!address) {
    return false;
  }

  const host = extractHost(address);

  if (host === 'localhost') {
    return true;
  }

  if (host === '::1') {
    return true;
  }

  if (host.startsWith('127.')) {
    return true;
  }

  return false;
}

// Treats all localhost addresses (127.x.x.x, ::1, localhost) as equivalent
export function addressesMatch(addr1: string, addr2: string): boolean {
  const host1 = extractHost(addr1);
  const host2 = extractHost(addr2);

  if (isLocalhost(host1) && isLocalhost(host2)) {
    return true;
  }

  return host1 === host2;
}