import { render, screen, userEvent } from 'design/utils/testing';

import cfg from 'e-teleport/config';
import { createTeleportContextE } from 'e-teleport/mocks/contexts';
import { TeleportProviderBasicE } from 'e-teleport/mocks/providers';
import { SessionSummariesUnifiedResourcesCta } from 'e-teleport/UnifiedResources/SessionSummariesUnifiedResourcesCta';
import { allAccessAcl, noAccess } from 'teleport/mocks/contexts';
import { KeysEnum } from 'teleport/services/storageService';
import type { Acl } from 'teleport/services/user/types';

let originalSessionSummariesEntitlement: (typeof cfg.oss.entitlements)['SessionSummaries'];
let originalHideInaccessibleFeatures: boolean;
let originalSessionSummarizerEnabled: boolean;

beforeEach(() => {
  localStorage.clear();

  originalSessionSummariesEntitlement = cfg.oss.entitlements.SessionSummaries;
  originalHideInaccessibleFeatures = cfg.oss.entitlements.FeatureHiding.enabled;
  originalSessionSummarizerEnabled = cfg.oss.sessionSummarizerEnabled;

  cfg.oss.entitlements.SessionSummaries = { enabled: true, limit: 0 };
  cfg.oss.entitlements.FeatureHiding.enabled = false;
  cfg.oss.sessionSummarizerEnabled = false;
});

afterEach(() => {
  cfg.oss.entitlements.SessionSummaries = originalSessionSummariesEntitlement;
  cfg.oss.entitlements.FeatureHiding.enabled = originalHideInaccessibleFeatures;
  cfg.oss.sessionSummarizerEnabled = originalSessionSummarizerEnabled;
});

function renderCta(
  overrideAcl?: Partial<Acl>,
  { resourceCount = 1, isFilterApplied = false } = {}
) {
  const ctx = createTeleportContextE({
    customAcl: { ...allAccessAcl, ...overrideAcl },
  });

  return render(
    <TeleportProviderBasicE teleportCtx={ctx}>
      <SessionSummariesUnifiedResourcesCta
        resourceCount={resourceCount}
        isFilterApplied={isFilterApplied}
      />
    </TeleportProviderBasicE>
  );
}

test('renders the dialog when Session Summaries is licensed and not previously seen', () => {
  renderCta();

  expect(
    screen.getByText(/Speed up audit reviews with Session Recording Summaries/)
  ).toBeInTheDocument();
});

test('does not render when Session Summaries is not licensed', () => {
  cfg.oss.entitlements.SessionSummaries = { enabled: false, limit: 0 };

  renderCta();

  expect(
    screen.queryByText(
      /Speed up audit reviews with Session Recording Summaries/
    )
  ).not.toBeInTheDocument();
});

test('does not render when hideInaccessibleFeatures is true', () => {
  cfg.oss.entitlements.FeatureHiding.enabled = true;

  renderCta();

  expect(
    screen.queryByText(
      /Speed up audit reviews with Session Recording Summaries/
    )
  ).not.toBeInTheDocument();
});

test('does not render when user lacks session summaries permissions', () => {
  renderCta({
    inferencePolicy: noAccess,
    inferenceModel: noAccess,
    inferenceSecret: noAccess,
  });

  expect(
    screen.queryByText(
      /Speed up audit reviews with Session Recording Summaries/
    )
  ).not.toBeInTheDocument();
});

test('does not render when the cluster is empty and no filter is applied', () => {
  renderCta(undefined, { resourceCount: 0, isFilterApplied: false });

  expect(
    screen.queryByText(
      /Speed up audit reviews with Session Recording Summaries/
    )
  ).not.toBeInTheDocument();
});

test('renders when there are no results but a filter is applied', () => {
  renderCta(undefined, { resourceCount: 0, isFilterApplied: true });

  expect(
    screen.getByText(/Speed up audit reviews with Session Recording Summaries/)
  ).toBeInTheDocument();
});

test('does not render when the session summarizer is already enabled', () => {
  cfg.oss.sessionSummarizerEnabled = true;

  renderCta();

  expect(
    screen.queryByText(
      /Speed up audit reviews with Session Recording Summaries/
    )
  ).not.toBeInTheDocument();
});

test('does not render when already seen', () => {
  localStorage.setItem(
    KeysEnum.IDENTITY_SECURITY_RECOMMENDATIONS_UNIFIED_RESOURCES_CTA_SEEN,
    'true'
  );

  renderCta();

  expect(
    screen.queryByText(
      /Speed up audit reviews with Session Recording Summaries/
    )
  ).not.toBeInTheDocument();
});

test('dismisses when clicking the close button', async () => {
  const user = userEvent.setup();

  renderCta();

  expect(
    screen.getByText(/Speed up audit reviews with Session Recording Summaries/)
  ).toBeInTheDocument();

  await user.click(screen.getByRole('button', { name: /close/i }));

  expect(
    screen.queryByText(
      /Speed up audit reviews with Session Recording Summaries/
    )
  ).not.toBeInTheDocument();
});

test('dismisses when clicking "Got it"', async () => {
  const user = userEvent.setup();

  renderCta();

  expect(
    screen.getByText(/Speed up audit reviews with Session Recording Summaries/)
  ).toBeInTheDocument();

  await user.click(screen.getByText('Got it'));

  expect(
    screen.queryByText(
      /Speed up audit reviews with Session Recording Summaries/
    )
  ).not.toBeInTheDocument();
});

test('renders the "Go to Session Recordings" link', () => {
  renderCta();

  const link = screen.getByRole('link', { name: 'Go to Session Recordings' });
  expect(link).toBeInTheDocument();
});

test('renders the AI disclaimer', () => {
  renderCta();

  expect(
    screen.getByText(/AI-powered session recording can make mistakes/)
  ).toBeInTheDocument();
});
