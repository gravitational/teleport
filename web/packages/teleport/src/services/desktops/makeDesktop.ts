import type { Desktop, WindowsDesktopService } from './types';

export function makeDesktop(json): Desktop {
  const { os, name, addr, host_id } = json;

  const labels = json.labels || [];

  return {
    os,
    name,
    addr,
    labels,
    host_id,
  };
}

export function makeDesktopService(json): WindowsDesktopService {
  const { name, hostname, addr } = json;

  const labels = json.labels || [];

  return {
    hostname,
    addr,
    labels,
    name,
  };
}
