import { ButtonBorder, Flex } from 'design';
import { Cli, Earth, Trash } from 'design/Icon';
import { HoverTooltip } from 'design/Tooltip/HoverTooltip';
import { MenuIcon, MenuItem } from 'shared/components/MenuAction';

import { Beam } from 'e-teleport/services/beams/types';
import cfg from 'teleport/config';
import { openNewTab } from 'teleport/lib/util';
import useTeleport from 'teleport/useTeleport';

import { BEAM_SSH_LOGIN, isBeamOwner, isProvisioning } from './constants';
import { usePublishBeam, useUnpublishBeam } from './useBeamMutations';

export function BeamRowActions({
  beam,
  clusterId,
  canEdit,
  canRemove,
  onRequestDelete,
}: {
  beam: Beam;
  clusterId: string;
  canEdit: boolean;
  canRemove: boolean;
  onRequestDelete: (beam: Beam) => void;
}) {
  const ctx = useTeleport();
  const publish = usePublishBeam(clusterId, beam);
  const unpublish = useUnpublishBeam(clusterId, beam);

  const provisioning = isProvisioning(beam);
  // TODO(nibrasohin): isOwner is a frontend approximation of who can connect/publish a
  // beam, but the real check lives in the backend. Revisit once the
  // beams API exposes per-action permissions so the UI can stop computing
  // this here.
  const isOwner = isBeamOwner(beam, ctx.storeUser.getUsername());
  const isPublished = !!beam.publish;
  const busy = publish.isPending || unpublish.isPending;

  const showOptionsMenu = (canEdit || canRemove) && !provisioning;
  const publishVerb = isPublished ? 'unpublish' : 'publish';
  const publishLabel = isPublished ? 'Unpublish Beam' : 'Publish Beam';

  const connectTooltip = !isOwner
    ? "You don't have permission to connect to this beam"
    : provisioning
      ? 'This beam is still provisioning. Try again shortly.'
      : '';

  const handleConnect = () =>
    openNewTab(
      cfg.getSshConnectRoute({
        clusterId,
        serverId: beam.node_id,
        login: BEAM_SSH_LOGIN,
      })
    );

  const handlePublishClick = () =>
    isPublished ? unpublish.mutate() : publish.mutate();

  const connectButton = (
    <HoverTooltip tipContent={connectTooltip}>
      <ButtonBorder
        size="small"
        disabled={!isOwner || provisioning}
        onClick={handleConnect}
      >
        <Cli size="small" mr={2} />
        Connect
      </ButtonBorder>
    </HoverTooltip>
  );

  const publishMenuItem = canEdit ? (
    isOwner ? (
      <MenuItem onClick={handlePublishClick} disabled={busy}>
        <Earth size="small" mr={2} />
        {publishLabel}
      </MenuItem>
    ) : (
      <HoverTooltip
        tipContent={`You don't have permission to ${publishVerb} this beam`}
      >
        <MenuItem disabled>
          <Earth size="small" mr={2} />
          {publishLabel}
        </MenuItem>
      </HoverTooltip>
    )
  ) : null;

  const deleteMenuItem = canRemove && (
    <MenuItem onClick={() => onRequestDelete(beam)} disabled={busy}>
      <Trash size="small" mr={2} />
      Delete...
    </MenuItem>
  );

  const optionsMenu = showOptionsMenu && (
    <MenuIcon tooltip="Options" buttonIconProps={{ size: 1 }}>
      {publishMenuItem}
      {deleteMenuItem}
    </MenuIcon>
  );

  return (
    <Flex gap={2} alignItems="center" justifyContent="flex-end">
      {connectButton}
      {optionsMenu}
    </Flex>
  );
}
