import teleportContextE from 'e-teleport/teleportContextE';
import useTeleport from 'teleport/useTeleport';

export default function useTeleportE() {
  return useTeleport() as teleportContextE;
}
