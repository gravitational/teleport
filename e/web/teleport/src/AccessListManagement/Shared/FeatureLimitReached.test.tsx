import { render, screen } from 'design/utils/testing';

import TeleportContextProvider from 'teleport/TeleportContextProvider';
import { createTeleportContext } from 'teleport/mocks/contexts';

import { FeatureLimitBlurb } from 'e-teleport/AccessListManagement/Shared/FeatureLimitReached';

test('defaults to 1 access list', () => {
  const ctx = createTeleportContext();
  ctx.isEnterprise = true;

  render(
    <TeleportContextProvider ctx={ctx}>
      <FeatureLimitBlurb />
    </TeleportContextProvider>
  );

  expect(
    screen.getByText('Your current plan supports 1 free Access List.')
  ).toBeInTheDocument();
});

test('list text is pluralized if count > 1', () => {
  const ctx = createTeleportContext();
  ctx.isEnterprise = true;

  render(
    <TeleportContextProvider ctx={ctx}>
      <FeatureLimitBlurb limit={2} />
    </TeleportContextProvider>
  );

  expect(
    screen.getByText('Your current plan supports 2 free Access Lists.')
  ).toBeInTheDocument();
});

test('limit of 0 returns empty; no warning (unlimited use)', () => {
  const ctx = createTeleportContext();
  ctx.isEnterprise = true;

  const { container } = render(
    <TeleportContextProvider ctx={ctx}>
      <FeatureLimitBlurb limit={0} />
    </TeleportContextProvider>
  );

  expect(container).toBeEmptyDOMElement();
});
