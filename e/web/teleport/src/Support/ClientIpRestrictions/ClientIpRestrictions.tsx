import { ChangeEvent, useMemo, useRef, useState } from 'react';
import { useTheme } from 'styled-components';

import { Alert } from 'design/Alert';
import Box from 'design/Box';
import {
  Button,
  ButtonPrimary,
  ButtonSecondary,
  ButtonWarning,
} from 'design/Button';
import Flex from 'design/Flex';
import {
  CirclePlay,
  CircleStop,
  ListAddCheck,
  Refresh,
  ShieldCheck,
} from 'design/Icon';
import { Indicator } from 'design/Indicator';
import { TextArea } from 'design/TextArea';
import { Theme } from 'design/theme';
import { HoverTooltip } from 'design/Tooltip';
import {
  InfoExternalTextLink,
  InfoGuideButton,
  InfoParagraph,
  InfoUl,
} from 'shared/components/SlidingSidePanel/InfoGuide';
import { useInterval } from 'shared/hooks/useInterval';

import { SupportSectionCard } from 'teleport/Support/Support';

import { DeactivateDialog } from './DeactivateDialog';
import { StatusBanner } from './StatusBanner';
import { useClientIpRestriction } from './useClientIpRestriction';
import { CirUiState, getRemainingMs, writeErrorMessage } from './utils';

const EDITOR_PLACEHOLDER = `Enter one CIDR block per line, e.g.:
100.20.56.0/32
104.18.0.0/16`;

const EMPTY_LIST_TIP = 'Add at least one CIDR block first';
const EMPTY_WHILE_ENFORCED_TIP =
  'Add at least one CIDR block, or select Deactivate to stop enforcing';

const parseCidrs = (text: string): string[] =>
  text
    .split('\n')
    .map(s => s.trim())
    .filter(s => s !== '');

export const ClientIpRestrictions = ({ clusterId }: { clusterId: string }) => {
  const theme = useTheme();
  // Bumped when a deadline elapses: a refetch returning identical data would not
  // re-render, leaving a frozen 0:00 countdown.
  const [now, setNow] = useState(() => Date.now());
  const {
    access,
    cir,
    uiState,
    isLoading,
    error,
    refetch,
    saving,
    saveError,
    clearSaveError,
    actions,
  } = useClientIpRestriction(clusterId, now);

  const canEdit = access.edit && access.create;

  const [editing, setEditing] = useState(false);
  const [buffer, setBuffer] = useState('');
  // The buffer survives background polls, so the eventual save must carry the
  // revision the buffer was based on, not the latest polled one -— otherwise it
  // silently overwrites what someone else saved mid-edit instead of failing
  // with the stale-revision conflict.
  const [editBaseRevision, setEditBaseRevision] = useState<string>();
  const [showDeactivate, setShowDeactivate] = useState(false);

  const savedContent = useMemo(() => (cir?.cidrs ?? []).join('\n'), [cir]);
  const editorContent = editing ? buffer : savedContent;

  const beginEdit = () => {
    setBuffer(savedContent);
    setEditBaseRevision(cir?.revision);
    setEditing(true);
  };
  const cancelEdit = () => setEditing(false);

  const handleBufferChange = (e: ChangeEvent<HTMLInputElement>) =>
    setBuffer(e.target.value);

  const onDeadlinePassed = () => {
    setNow(Date.now());
    refetch();
  };

  const onSaveEdit = async () => {
    const cidrs = parseCidrs(buffer);
    const listUnchanged =
      cidrs.join('\n') === parseCidrs(savedContent).join('\n');

    // An edit that changes nothing would only bump the revision (and, from active,
    // cycle through pending). The excluded states are the ones where saving has a
    // side effect of its own: it stops a rollout or a test run, or settles back to
    // draft.
    const wouldChangeNothing =
      listUnchanged &&
      !cir?.expires &&
      (uiState === 'draft' ||
        uiState === 'active' ||
        (uiState === 'notConfigured' && cidrs.length === 0));
    if (wouldChangeNothing) {
      setEditing(false);
      return;
    }

    // Editing while enforced re-enforces the new list; otherwise it saves a draft,
    // which stops whatever was in flight, since the edited list is not what was.
    const enforcing = uiState === 'active';
    const stopsSomething =
      uiState === 'pending' ||
      uiState === 'testRunApplying' ||
      uiState === 'testRunActive' ||
      uiState === 'testRunEnding';

    let message = 'Draft saved';
    if (enforcing) {
      message = 'Allowlist updated';
    } else if (uiState === 'pending') {
      message = 'Draft saved. The rollout was stopped, so nothing is enforced.';
    } else if (stopsSomething) {
      message =
        'Draft saved and the test run ended. Start a new test run to try the updated allowlist.';
    }

    const ok = enforcing
      ? await actions.apply(cidrs, message, editBaseRevision)
      : await actions.saveDraft(cidrs, message, editBaseRevision);
    // Only leave the editor on success, or the typed list is lost.
    if (ok) {
      setEditing(false);
    }
  };

  const onDeactivate = async () => {
    if (await actions.deactivate('Returning the allowlist to draft')) {
      setShowDeactivate(false);
    }
  };

  if (!access.read) {
    return null;
  }

  const busy = saving;
  const showEditor = uiState !== 'unknown' || editing || !!cir?.cidrs.length;
  // An empty allowlist allows all traffic, but the server still reports it as
  // active, so the panel would claim the cluster is restricted while it is wide
  // open. Enforcing one is blocked rather than explained after the fact.
  const hasCidrs = editing
    ? parseCidrs(buffer).length > 0
    : (cir?.cidrs.length ?? 0) > 0;

  return (
    // The card owns its heading: it renders the icon and title itself, and takes
    // the column span as a style prop.
    <SupportSectionCard
      title="IP Allowlist"
      icon={<ListAddCheck />}
      titleAction={<InfoGuideButton config={{ guide: <InfoGuide /> }} />}
      gridColumn="1 / -1"
      transition="0.2s"
    >
      {isLoading ? (
        <Box textAlign="center" p={4}>
          <Indicator />
        </Box>
      ) : error ? (
        // Inline rather than a toast: there is no content behind it.
        <Alert
          kind="danger"
          details={error.message}
          primaryAction={{ content: 'Retry', onClick: () => refetch() }}
        >
          Failed to load the IP allowlist
        </Alert>
      ) : (
        <>
          {/* Kept while editing: it still describes what is deployed. */}
          <Box mb={3}>
            <StatusBanner
              state={uiState}
              countdown={
                // Also during the rollout: the window is already draining then.
                (uiState === 'testRunActive' ||
                  uiState === 'testRunApplying') &&
                cir?.expires ? (
                  <Countdown
                    key={cir.expires}
                    expires={cir.expires}
                    onExpire={onDeadlinePassed}
                  />
                ) : undefined
              }
            />
          </Box>

          {showEditor && (
            <Flex flex="1" data-testid="text-editor-container">
              <TextArea
                disabled={!editing || busy}
                bg={!editing ? 'buttons.bgDisabled' : 'levels.sunken'}
                value={editorContent}
                placeholder={EDITOR_PLACEHOLDER}
                style={{
                  maxHeight: 240,
                  minHeight: 36,
                  height: getEditorHeight(editorContent, theme),
                }}
                onChange={handleBufferChange}
                name="allowlist"
                aria-label="Allowed CIDR blocks, one per line"
              />
            </Flex>
          )}

          <Flex mt={3} gap={2} flexWrap="wrap">
            <ActionBar
              uiState={uiState}
              editing={editing}
              canEdit={canEdit}
              busy={busy}
              hasCidrs={hasCidrs}
              onEdit={beginEdit}
              onSaveEdit={onSaveEdit}
              onCancelEdit={cancelEdit}
              onApply={() => actions.apply()}
              onTestRun={() => actions.startTestRun()}
              onConfirm={() => actions.confirm()}
              onCancel={() => actions.cancel()}
              onDeactivate={() => {
                // saveError outlives its write, so clear it before showing it.
                clearSaveError();
                setShowDeactivate(true);
              }}
              onRefresh={() => refetch()}
            />
          </Flex>
        </>
      )}

      {showDeactivate && (
        <DeactivateDialog
          processing={busy}
          error={saveError ? writeErrorMessage(saveError) : undefined}
          onConfirm={onDeactivate}
          onCancel={() => setShowDeactivate(false)}
        />
      )}
    </SupportSectionCard>
  );
};

/**
 * ActionBar renders the buttons available for the current UI state. All write
 * actions are gated by canEdit and disabled while a write is in flight.
 */
function ActionBar({
  uiState,
  editing,
  canEdit,
  busy,
  hasCidrs,
  onEdit,
  onSaveEdit,
  onCancelEdit,
  onApply,
  onTestRun,
  onConfirm,
  onCancel,
  onDeactivate,
  onRefresh,
}: {
  uiState: CirUiState;
  editing: boolean;
  canEdit: boolean;
  busy: boolean;
  hasCidrs: boolean;
  onEdit: () => void;
  onSaveEdit: () => void;
  onCancelEdit: () => void;
  onApply: () => void;
  onTestRun: () => void;
  onConfirm: () => void;
  onCancel: () => void;
  onDeactivate: () => void;
  onRefresh: () => void;
}) {
  if (editing) {
    // Emptying the list while it is enforced would open the cluster to every IP,
    // so it has to go through Deactivate, which asks for confirmation.
    const blockedEmpty = uiState === 'active' && !hasCidrs;
    return (
      <>
        <HoverTooltip tipContent={blockedEmpty ? EMPTY_WHILE_ENFORCED_TIP : ''}>
          <ButtonPrimary disabled={busy || blockedEmpty} onClick={onSaveEdit}>
            Save
          </ButtonPrimary>
        </HoverTooltip>
        <ButtonSecondary disabled={busy} onClick={onCancelEdit}>
          Cancel
        </ButtonSecondary>
      </>
    );
  }

  // Read-only viewers get no write controls (a refresh is always safe).
  if (!canEdit) {
    return <RefreshButton busy={busy} onClick={onRefresh} />;
  }

  switch (uiState) {
    case 'notConfigured':
      return <EditButton busy={busy} onClick={onEdit} />;
    // Nothing is enforced (draft, expired) or it is on its way out
    // (returningToDraft), so both ways forward re-apply the list. Neither re-sends
    // a lapsed expiry, since the payloads are built fresh.
    case 'draft':
    case 'expired':
    case 'returningToDraft':
      return (
        <>
          <EditButton busy={busy} onClick={onEdit} />
          <EnforceButton busy={busy} hasCidrs={hasCidrs} onClick={onApply} />
          <TestRunButton busy={busy} hasCidrs={hasCidrs} onClick={onTestRun} />
        </>
      );
    // Cancel writes draft, which stops the rollout; Test Run only puts a deadline on
    // the rollout already under way.
    case 'pending':
      return (
        <>
          <EditButton busy={busy} onClick={onEdit} />
          <TestRunButton busy={busy} hasCidrs={hasCidrs} onClick={onTestRun} />
          <CancelRolloutButton busy={busy} onClick={onCancel} />
        </>
      );
    // Edit is offered here: a missing IP tends to surface during a test run.
    case 'testRunApplying':
      return (
        <>
          <EditButton busy={busy} onClick={onEdit} />
          <StopTestButton busy={busy} onClick={onCancel} />
        </>
      );
    // Still enforced until the rules are removed, so Confirm still applies.
    case 'testRunActive':
    case 'testRunEnding':
      return (
        <>
          <EditButton busy={busy} onClick={onEdit} />
          <ConfirmButton busy={busy} onClick={onConfirm} />
          <StopTestButton busy={busy} onClick={onCancel} />
        </>
      );
    case 'active':
      return (
        <>
          <EditButton busy={busy} onClick={onEdit} />
          <DeactivateButton busy={busy} onClick={onDeactivate} />
        </>
      );
    case 'unknown':
    default:
      return <RefreshButton busy={busy} onClick={onRefresh} />;
  }
}

type ActionProps = { busy: boolean; onClick: () => void };

/** Applying an empty allowlist would restrict nothing while reporting active. */
type GatedActionProps = ActionProps & { hasCidrs: boolean };

// Filled: an outlined button next to the disabled editor reads as disabled.
const EditButton = ({ busy, onClick }: ActionProps) => (
  <ButtonPrimary disabled={busy} onClick={onClick}>
    Edit
  </ButtonPrimary>
);

const EnforceButton = ({ busy, hasCidrs, onClick }: GatedActionProps) => (
  <HoverTooltip tipContent={hasCidrs ? '' : EMPTY_LIST_TIP}>
    <Button
      gap={2}
      intent="success"
      disabled={busy || !hasCidrs}
      onClick={onClick}
    >
      <ShieldCheck size="small" />
      Enforce
    </Button>
  </HoverTooltip>
);

const TestRunButton = ({ busy, hasCidrs, onClick }: GatedActionProps) => (
  <HoverTooltip tipContent={hasCidrs ? '' : EMPTY_LIST_TIP}>
    <ButtonSecondary gap={2} disabled={busy || !hasCidrs} onClick={onClick}>
      <CirclePlay size="small" />
      Test Run
    </ButtonSecondary>
  </HoverTooltip>
);

const ConfirmButton = ({ busy, onClick }: ActionProps) => (
  <Button gap={2} intent="success" disabled={busy} onClick={onClick}>
    <ShieldCheck size="small" />
    Confirm
  </Button>
);

/** The same write as Stop test, labelled for what it interrupts. */
const CancelRolloutButton = ({ busy, onClick }: ActionProps) => (
  <ButtonSecondary gap={2} disabled={busy} onClick={onClick}>
    <CircleStop size="small" />
    Cancel
  </ButtonSecondary>
);

const StopTestButton = ({ busy, onClick }: ActionProps) => (
  <ButtonWarning gap={2} disabled={busy} onClick={onClick}>
    <CircleStop size="small" />
    Stop test
  </ButtonWarning>
);

const DeactivateButton = ({ busy, onClick }: ActionProps) => (
  <ButtonSecondary gap={2} disabled={busy} onClick={onClick}>
    <Refresh size="small" />
    Deactivate
  </ButtonSecondary>
);

const RefreshButton = ({ busy, onClick }: ActionProps) => (
  <ButtonSecondary gap={2} disabled={busy} onClick={onClick}>
    <Refresh size="small" />
    Refresh
  </ButtonSecondary>
);

function formatRemaining(remainingMs: number): string {
  const totalSeconds = Math.ceil(remainingMs / 1000);
  const minutes = Math.floor(totalSeconds / 60);
  if (minutes < 60) {
    const seconds = totalSeconds % 60;
    return `${minutes}:${seconds.toString().padStart(2, '0')}`;
  }
  const hours = Math.floor(minutes / 60);
  if (hours < 24) {
    return `${hours}h ${minutes % 60}m`;
  }
  const days = Math.floor(hours / 24);
  return `${days}d ${hours % 24}h`;
}

/**
 * Time remaining until `expires`, ticking once a second, calling `onExpire` once
 * at zero. Its own component so the per-second re-render stays off the panel.
 *
 * Give it `key={expires}` so a new deadline remounts it rather than needing the
 * state and the fired guard reset by hand.
 *
 * Exported for its tests; nothing else renders it.
 */
export function Countdown({
  expires,
  onExpire,
}: {
  expires: string;
  onExpire?: () => void;
}) {
  const [remainingMs, setRemainingMs] = useState(() => getRemainingMs(expires));
  const firedRef = useRef(false);

  useInterval(() => {
    const next = getRemainingMs(expires);
    setRemainingMs(next);
    if (next <= 0 && !firedRef.current) {
      firedRef.current = true;
      onExpire?.();
    }
  }, 1000);

  return <>{formatRemaining(remainingMs)}</>;
}

const getEditorHeight = (editorContent: string, theme: Theme): number => {
  const editorPadding = theme.space[4];
  const editorPxPerLine = theme.fontSizes[7];
  const qtyLines =
    editorContent === ''
      ? EDITOR_PLACEHOLDER.split('\n').length
      : editorContent?.split('\n').length;
  return qtyLines * editorPxPerLine + editorPadding;
};

const InfoGuide = () => (
  <Box>
    <InfoParagraph>
      <b>Client IP Allowlist</b> restricts access to your Teleport Cloud
      cluster, allowing traffic only from the specified network ranges (CIDR
      blocks).
    </InfoParagraph>

    <InfoParagraph>
      <b>Draft</b> is where an allowlist starts: saved but not enforced, so you
      can get the list right without risk. Keep it in draft as long as you like,
      then enforce it when you need it.
    </InfoParagraph>

    <InfoParagraph>
      <b>Enforce</b> applies it with no time limit: <b>pending</b> while
      Teleport Cloud pushes it to the network layer, then <b>active</b> until
      you deactivate it.
    </InfoParagraph>

    <InfoParagraph>
      <b>Test Run</b> enforces it the same way, but for 30 minutes only — the
      safe way to try a list you are unsure about. If it locks you out, wait for
      it to <b>expire</b>: enforcement stops, the cluster is reachable from any
      IP again, and your allowlist is still saved.
    </InfoParagraph>

    <InfoParagraph>
      During a test run, <b>Confirm</b> keeps it enforced with no timer and{' '}
      <b>Stop test</b> ends it now. Editing the list also ends the run, saving
      your changes as a draft so you can test the corrected list.
    </InfoParagraph>

    <InfoParagraph>Changes are recorded in the audit log.</InfoParagraph>

    <InfoParagraph>Add one CIDR block per line, without commas.</InfoParagraph>

    <InfoParagraph>
      Changes take effect in 5–15 minutes and will terminate existing
      connections. Maintenance windows are ignored.
    </InfoParagraph>

    <InfoParagraph>
      <b>Limitations and Notes</b>
      <InfoUl>
        <li>
          Misconfiguration can block all access to your cluster. Make sure to
          include your current network before enforcing changes.
        </li>
        <li>
          Teleport does not auto-add third-party service ranges. Add all
          required CIDRs explicitly.
        </li>
        <li>
          The allowlist applies to Teleport Cloud access; it does not replace
          your organization’s network/firewall policies.
        </li>
        <li>
          Up to 256 CIDR blocks are allowed. If you need more,{' '}
          <InfoExternalTextLink href="https://support.goteleport.com">
            create a Support Ticket
          </InfoExternalTextLink>
        </li>
      </InfoUl>
    </InfoParagraph>

    <InfoParagraph>
      Not sure what a CIDR block is? See{' '}
      <InfoExternalTextLink href="https://en.wikipedia.org/wiki/Classless_Inter-Domain_Routing">
        this short primer
      </InfoExternalTextLink>
      .
    </InfoParagraph>
  </Box>
);
