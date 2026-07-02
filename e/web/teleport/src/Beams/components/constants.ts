import { Beam, ComputeStatus } from 'e-teleport/services/beams/types';

export const PROVISION_COMPLETE: ComputeStatus = 'provision_complete';

// The SSH login used to connect to a beam.
export const BEAM_SSH_LOGIN = 'beams';

// A beam is provisioning until both its compute status reports complete and
// the node id has been populated. Connect/publish/open are gated on this.
export function isProvisioning(beam: Beam): boolean {
  return beam.compute_status !== PROVISION_COMPLETE || !beam.node_id;
}

export function isBeamOwner(beam: Beam, currentUsername: string): boolean {
  return beam.user === currentUsername;
}

// Builds the public URL Teleport assigns to a published beam app.
// URL parsing normalises the host so the default https port (443) is stripped
// while non-default ports (e.g. 3080 on dev clusters) are preserved.
export function publishedBeamUrl(beam: Beam, clusterPublicUrl: string): string {
  if (!beam.app_name || !beam.publish) return '';
  const host = new URL(`https://${clusterPublicUrl}`).host;
  if (beam.publish.protocol === 'tcp') {
    return `tcp://${beam.app_name}.${host}:${beam.publish.port}`;
  }
  return `https://${beam.app_name}.${host}`;
}
