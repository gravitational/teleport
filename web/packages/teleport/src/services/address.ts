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