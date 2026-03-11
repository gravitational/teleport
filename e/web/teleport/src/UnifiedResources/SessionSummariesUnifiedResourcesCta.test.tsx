import { render, screen, userEvent } from 'design/utils/testing';

import cfg from 'e-teleport/config';
import { createTeleportContextE } from 'e-teleport/mocks/contexts';
import { TeleportProviderBasicE } from 'e-teleport/mocks/providers';
import { SessionSummariesUnifiedResourcesCta } from 'e-teleport/UnifiedResources/SessionSummariesUnifiedResourcesCta';
import { allAccessAcl, noAccess } from 'teleport/mocks/contexts';
import { KeysEnum } from 'teleport/services/storageService';
import type { Acl } from 'teleport/services/user/types';

let originalIdentitySecurityLicensed: boolean;
let originalHideInaccessibleFeatures: boolean;

beforeEach(() => {
  localStorage.clear();

  originalIdentitySecurityLicensed = cfg.oss.identitySecurity.licensed;
  originalHideInaccessibleFeatures = cfg.oss.hideInaccessibleFeatures;

  cfg.oss.identitySecurity.licensed = true;
  cfg.oss.hideInaccessibleFeatures = false;
});

afterEach(() => {
  cfg.oss.identitySecurity.licensed = originalIdentitySecurityLicensed;
  cfg.oss.hideInaccessibleFeatures = originalHideInaccessibleFeatures;
});

function renderCta(overrideAcl?: Partial<Acl>) {
  const ctx = createTeleportContextE({
    customAcl: { ...allAccessAcl, ...overrideAcl },
  });

  return render(
    <TeleportProviderBasicE teleportCtx={ctx}>
      <SessionSummariesUnifiedResourcesCta />
    </TeleportProviderBasicE>
  );
}

test('renders the dialog when identity security is licensed and not previously seen', () => {
  renderCta();

  expect(
    screen.getByText(/Speed up audit reviews with Session Recording Summaries/)
  ).toBeInTheDocument();
});

test('does not render when identity security is not licensed', () => {
  cfg.oss.identitySecurity.licensed = false;

  renderCta();

  expect(
    screen.queryByText(
      /Speed up audit reviews with Session Recording Summaries/
    )
  ).not.toBeInTheDocument();
});

test('does not render when hideInaccessibleFeatures is true', () => {
  cfg.oss.hideInaccessibleFeatures = true;

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
