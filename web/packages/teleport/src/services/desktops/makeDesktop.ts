import { Desktop } from './types';

export default function makeDesktop(json): Desktop {
  const { os, name, addr } = json;

  const labels = json.labels || [];

  return {
    os,
    name,
    addr,
    labels,
  };
}
