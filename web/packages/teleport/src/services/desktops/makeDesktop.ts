import { Desktop } from './types';

export default function makeDesktop(json): Desktop {
  const { os, name, addr } = json;
  return { os, name, addr };
}
