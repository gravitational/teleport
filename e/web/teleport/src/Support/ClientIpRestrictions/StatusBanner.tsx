import { ReactNode } from 'react';
import styled, { css, useTheme } from 'styled-components';

import Box from 'design/Box';
import {
  Clock,
  ShieldCheck,
  ShieldWarning,
  SyncAlt,
  Warning,
} from 'design/Icon';
import { IconProps } from 'design/Icon/Icon';
import { rotate360 } from 'design/keyframes';
import { StatusKind } from 'design/Status';
import { getVariantColors } from 'design/Status/statusColors';
import { P2 } from 'design/Text';

/**
 * The states the panel can be in. Every one of them has a banner, so the set
 * lives here, with the copy that gives each its meaning.
 */
export type CirUiState =
  | 'notConfigured'
  | 'draft'
  | 'pending'
  | 'returningToDraft'
  | 'testRunApplying'
  | 'testRunActive'
  | 'testRunEnding'
  | 'expired'
  | 'active'
  | 'unknown';

type BannerConfig = {
  kind: StatusKind;
  icon: React.ComponentType<IconProps>;
  /** The server is actively working through this state. */
  spinning?: boolean;
  title: string;
  /** A line each: what it means for traffic now, then what to do about it. */
  details: [string, string];
};

/**
 * Titles use the same words as the resource, so the panel, `tctl`, Terraform and
 * the docs all say the same thing.
 */
const bannerFor: Record<CirUiState, BannerConfig> = {
  notConfigured: {
    kind: 'neutral',
    icon: ShieldWarning,
    title: 'Not configured',
    details: [
      'No allowlist configured yet. Any IP can reach the cluster.',
      'Add the CIDR blocks you want to allow, then enforce the list to ensure only those IPs can access the cluster.',
    ],
  },
  draft: {
    kind: 'neutral',
    icon: ShieldWarning,
    title: 'Draft',
    details: [
      'Your allowlist is saved but not enforced, so any IP can still reach the cluster.',
      'Enforce it when you are ready to restrict access.',
    ],
  },
  pending: {
    // Not "info": that blue is loud next to the neutral and amber states.
    kind: 'primary',
    icon: SyncAlt,
    spinning: true,
    title: 'Pending',
    details: [
      'Teleport Cloud is applying your allowlist at the network layer. This takes 5 to 15 minutes, and access is unchanged until it finishes.',
      'Cancel or edit the list to stop the rollout and return it to draft.',
    ],
  },
  returningToDraft: {
    kind: 'primary',
    icon: SyncAlt,
    spinning: true,
    title: 'Returning to draft',
    details: [
      'Teleport Cloud is removing your allowlist from the network layer. This takes 5 to 15 minutes, and access is unchanged until it finishes.',
      'The allowlist will be stored as a draft, ready to enforce again.',
    ],
  },
  testRunApplying: {
    kind: 'warning',
    icon: SyncAlt,
    spinning: true,
    title: 'Test run pending',
    details: [
      'Teleport Cloud is applying your test-run allowlist at the network layer. If the list is wrong and you lose access, you will get it back automatically when the test run expires.',
      'Stop test ends the run immediately and returns the allowlist to draft.',
    ],
  },
  testRunActive: {
    kind: 'warning',
    icon: Clock,
    // Title is rendered with the live countdown appended (see below).
    title: 'Test run active',
    details: [
      'Your allowlist is enforced until the test run expires, then any IP can reach the cluster again.',
      'Check that your expected IPs can connect, then Confirm or Stop test.',
    ],
  },
  testRunEnding: {
    kind: 'warning',
    icon: SyncAlt,
    spinning: true,
    title: 'Test run expired',
    details: [
      'Your allowlist is still enforced while Teleport Cloud removes the rules. This takes a few minutes.',
      'Confirm to keep enforcing it instead.',
    ],
  },
  expired: {
    kind: 'neutral',
    icon: Clock,
    title: 'Expired',
    details: [
      'The test run expired unconfirmed, so nothing is enforced and any IP can reach the cluster.',
      'Your allowlist is still saved, so you can enforce it or start another test run whenever you are ready.',
    ],
  },
  active: {
    kind: 'success',
    icon: ShieldCheck,
    title: 'Active',
    details: [
      'Your allowlist is enforced, so only the IPs in it can reach the cluster.',
      'Deactivate to stop enforcing and return it to draft.',
    ],
  },
  unknown: {
    kind: 'neutral',
    icon: Warning,
    title: 'Unknown',
    details: [
      'We could not determine whether the allowlist is enforced.',
      'Refresh to try again.',
    ],
  },
};

const IconSlot = styled(Box)<{ spinning?: boolean }>`
  display: flex;
  align-items: center;
  flex-shrink: 0;
  height: ${({ theme }) => theme.typography.body2.lineHeight};
  ${({ spinning }) =>
    spinning &&
    css`
      animation: ${rotate360} 1.2s linear infinite;
    `}
`;

const StatusBlock = styled(Box)<{ kind: StatusKind }>`
  border-radius: ${({ theme }) => theme.radii[2]}px;
  border-left: 3px solid;
  padding: ${({ theme }) => theme.space[3]}px;
  display: flex;
  align-items: flex-start;
  gap: ${({ theme }) => theme.space[3]}px;

  ${({ kind, theme }) => {
    const { bg, border } = getVariantColors(theme, kind, 'filled-tonal');
    return css`
      border-left-color: ${border};
      background: ${bg};
    `;
  }}
`;

export function StatusBanner({
  state,
  countdown,
}: {
  state: CirUiState;
  countdown?: ReactNode;
}) {
  const theme = useTheme();
  const cfg = bannerFor[state];
  const Icon = cfg.icon;
  return (
    <StatusBlock kind={cfg.kind} role="status" aria-live="polite">
      <IconSlot spinning={cfg.spinning}>
        <Icon
          size="medium"
          color={getVariantColors(theme, cfg.kind, 'filled-tonal').iconColor}
        />
      </IconSlot>
      <Box flex="1" minWidth={0}>
        <P2 bold color="text.main" mb={1}>
          {cfg.title}
          {countdown != null && (
            <span aria-hidden="true"> — {countdown} remaining</span>
          )}
        </P2>
        {cfg.details.map((line, i) => (
          <P2 key={line} color="text.slightlyMuted" mt={i === 0 ? 0 : 1} mb={0}>
            {line}
          </P2>
        ))}
      </Box>
    </StatusBlock>
  );
}
