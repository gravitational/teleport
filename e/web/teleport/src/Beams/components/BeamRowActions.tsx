import {
  ButtonBorder,
  Flex,
  InfoIcon,
  TerminalIcon,
  Tooltip,
} from '@gravitational/design-system';
import { type ReactNode } from 'react';
import styled from 'styled-components';

import { MoreHoriz } from 'design/Icon';
import { Theme } from 'design/theme/themes/types';
import { copyToClipboard } from 'design/utils/copyToClipboard';
import { MenuIcon, MenuItem } from 'shared/components/MenuAction';
import { useToastNotifications } from 'shared/components/ToastNotification';

import { Beam } from 'e-teleport/services/beams/types';
import cfg from 'teleport/config';
import { openNewTab } from 'teleport/lib/util';
import useTeleport from 'teleport/useTeleport';

import {
  BEAM_SSH_LOGIN,
  isBeamOwner,
  isProvisioning,
  notifyError,
} from './constants';
import { usePublishBeam, useUnpublishBeam } from './useBeamMutations';

const TooltipLink = styled.a`
  color: ${({ theme }) => theme.colors.accent.main};
  text-decoration: underline;

  &:hover {
    color: ${({ theme }) => theme.colors.accent.hover};
  }
`;

const HTTP_PUBLISH_TOOLTIP = (
  <>
    Publishes the beam's contents as a web app (OSI L7) on port 8080. Use this
    to expose a web server, REST API, or UI running in the beam. Share via URL
    with authorized, authenticated users of your cluster.
  </>
);
const TCP_PUBLISH_TOOLTIP = (
  <>
    Publishes the beam's contents as a TCP app (OSI L4). Use this to connect to
    services which Teleport doesn't natively support, such as SMTP servers,
    Kafka, and gRPC. Also covers database protocols that are NOT{' '}
    <TooltipLink
      href="https://goteleport.com/docs/enroll-resources/database-access/faq/"
      target="_blank"
      rel="noreferrer"
    >
      natively supported
    </TooltipLink>{' '}
    by Teleport.
  </>
);
const UNPUBLISH_TOOLTIP =
  'Removes the published application so the beam is no longer reachable via its public URL.';

type PublishItem = {
  label: string;
  info: ReactNode;
  onClick: () => void;
};

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
  const toast = useToastNotifications();
  const publish = usePublishBeam(clusterId, beam);
  const unpublish = useUnpublishBeam(clusterId, beam);

  const handleCopyUuid = async () => {
    try {
      await copyToClipboard(beam.name);
      toast.add({
        severity: 'success',
        content: { title: 'UUID copied to clipboard' },
      });
    } catch (err) {
      notifyError(toast, 'Failed to copy UUID', err);
    }
  };

  const provisioning = isProvisioning(beam);
  const isOwner = isBeamOwner(beam, ctx.storeUser.getUsername());
  const isPublished = !!beam.publish;
  const busy = publish.isPending || unpublish.isPending;

  const showOptionsMenu = (canEdit || canRemove) && !provisioning;
  const publishVerb = isPublished ? 'unpublish' : 'publish';

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

  const connectButton = (
    <Tooltip content={connectTooltip}>
      <ConnectButton
        size="md"
        disabled={!isOwner || provisioning}
        onClick={handleConnect}
      >
        Connect / SSH
        <TerminalIcon boxSize={4} ml={2} />
      </ConnectButton>
    </Tooltip>
  );

  const renderPublishItem = ({ label, info, onClick }: PublishItem) =>
    isOwner ? (
      <BeamMenuItem key={label} onClick={onClick} disabled={busy}>
        {label}
        <PublishInfoSlot onClick={e => e.stopPropagation()}>
          <Tooltip interactive content={info}>
            <InfoIcon boxSize={4} color="text.slightlyMuted" />
          </Tooltip>
        </PublishInfoSlot>
      </BeamMenuItem>
    ) : (
      <Tooltip
        key={label}
        content={`You don't have permission to ${publishVerb} this beam`}
      >
        <BeamMenuItem disabled>{label}</BeamMenuItem>
      </Tooltip>
    );

  const publishItems: PublishItem[] = !canEdit
    ? []
    : isPublished
      ? [
          {
            label: 'Unpublish Beam',
            info: UNPUBLISH_TOOLTIP,
            onClick: () => unpublish.mutate(),
          },
        ]
      : [
          {
            label: 'Publish as HTTP',
            info: HTTP_PUBLISH_TOOLTIP,
            onClick: () => publish.mutate('http'),
          },
          {
            label: 'Publish as TCP',
            info: TCP_PUBLISH_TOOLTIP,
            onClick: () => publish.mutate('tcp'),
          },
        ];

  const optionsMenu = showOptionsMenu && (
    <MenuIcon
      tooltip="Options"
      Icon={MoreHoriz}
      buttonIconProps={{ css: borderedMenuTriggerCss, 'aria-label': 'Options' }}
    >
      {publishItems.map(renderPublishItem)}
      <BeamMenuItem onClick={handleCopyUuid}>Copy UUID</BeamMenuItem>
      {canRemove && (
        <BeamMenuItem onClick={() => onRequestDelete(beam)} disabled={busy}>
          Delete
        </BeamMenuItem>
      )}
    </MenuIcon>
  );

  return (
    <Flex gap={2} alignItems="center" justifyContent="flex-end">
      {connectButton}
      {optionsMenu}
    </Flex>
  );
}

const PublishInfoSlot = styled.span`
  display: inline-flex;
  align-items: center;
  margin-left: auto;
  padding-left: ${({ theme }) => theme.space[3]}px;
`;

const BeamMenuItem = styled(MenuItem).attrs({
  as: 'button',
  type: 'button',
})`
  width: 100%;
  background: transparent;
  border: none;
  font: inherit;
  text-align: left;
`;

const ConnectButton = styled(ButtonBorder)`
  padding-left: ${({ theme }) => theme.space[3]}px;
  padding-right: ${({ theme }) => theme.space[3]}px;
  color: ${({ theme }) => theme.colors.text.slightlyMuted};
`;

const borderedMenuTriggerCss = ({ theme }: { theme: Theme }) => `
  border: 1px solid ${theme.colors.interactive.tonal.neutral[2]};
  border-radius: ${theme.radii[2]}px;
  height: 32px;
  width: 32px;

  &:hover {
    border-color: ${theme.colors.text.slightlyMuted};
  }

  &:focus {
    outline: none;
  }

  &:focus-visible {
    outline: 2px solid ${theme.colors.brand};
    outline-offset: 2px;
  }
`;
