import useTeleport from 'teleport/useTeleport';

import teleportContextE from 'e-teleport/teleportContextE';

export default function useTeleportE() {
  return useTeleport() as teleportContextE;
}
