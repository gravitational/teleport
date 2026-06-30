import { userEvent } from '@testing-library/user-event';
import { ComponentProps } from 'react';

import {
  enableMswServer,
  render,
  screen,
  server,
  testQueryClient,
  waitFor,
} from 'design/utils/testing';

import { createTeleportContextE } from 'e-teleport/mocks/contexts';
import TeleportContextE from 'e-teleport/teleportContextE';
import {
  createEnrollPairingError,
  createEnrollPairingSuccess,
  getCurrentEnrollPairingError,
  getCurrentEnrollPairingSuccess,
} from 'e-teleport/test/helpers/enrollPairing';
import TeleportContextProvider from 'teleport/TeleportContextProvider';

import { EnrollMobileDeviceWizard } from './EnrollMobileDeviceWizard';

enableMswServer();

afterEach(async () => {
  await testQueryClient.resetQueries();
});

function makeWrapper({
  ctx = createTeleportContextE(),
  ...wizardProps
}: Partial<ComponentProps<typeof EnrollMobileDeviceWizard>> & {
  ctx?: TeleportContextE;
} = {}) {
  return (
    <TeleportContextProvider ctx={ctx}>
      <EnrollMobileDeviceWizard close={() => {}} {...wizardProps} />
    </TeleportContextProvider>
  );
}

test('renders the QR code once the pairing is created', async () => {
  server.use(createEnrollPairingSuccess());
  server.use(getCurrentEnrollPairingSuccess());

  render(makeWrapper());

  const image = await screen.findByRole('img');
  expect(image).toHaveAttribute('src', 'data:image/png;base64,base64-qr-code');
  expect(
    screen.getByText(/Scan the QR code on your mobile device/i)
  ).toBeInTheDocument();
});

test('surfaces a create error', async () => {
  server.use(createEnrollPairingError(500, 'boom'));

  render(makeWrapper());

  expect(
    await screen.findByText('Could not create an enrollment pairing')
  ).toBeInTheDocument();
  expect(screen.getByText('boom')).toBeInTheDocument();
});

test('surfaces a polling error', async () => {
  server.use(createEnrollPairingSuccess());
  server.use(getCurrentEnrollPairingError(500, 'poll boom'));

  render(makeWrapper());

  expect(
    await screen.findByText('Could not poll the enrollment pairing')
  ).toBeInTheDocument();
  expect(screen.getByText('poll boom')).toBeInTheDocument();
});

test('advances past the QR step when create returns a pairing past awaiting_device', async () => {
  // createEnrollPairing returned an existing pairing already in a later state,
  // e.g. the user opened the wizard before, the iOS app already pinged the
  // public RPC, and the user re-opened the wizard to approve. Polling is never
  // enabled and the wizard advances on the create result alone.
  server.use(
    createEnrollPairingSuccess({
      state: 'awaiting_approval',
      token: 'pairing-token',
    })
  );
  const ctx = createTeleportContextE();
  jest.spyOn(ctx.deviceService, 'getCurrentEnrollPairing');

  render(makeWrapper({ ctx }));

  // StepSlider keeps both steps in the DOM during the slide animation, so
  // only assert the new step appears rather than that the old one is gone.
  expect(await screen.findByText('Request received')).toBeInTheDocument();
  expect(ctx.deviceService.getCurrentEnrollPairing).not.toHaveBeenCalled();
});

test('advances past the QR step when the pairing transitions during polling', async () => {
  // create returns awaiting_device → polling enabled → first
  // poll returns awaiting_approval → wizard advances.
  server.use(createEnrollPairingSuccess());
  server.use(
    getCurrentEnrollPairingSuccess({
      state: 'awaiting_approval',
      token: 'pairing-token',
    })
  );

  render(makeWrapper());

  expect(await screen.findByText('Request received')).toBeInTheDocument();
});

test('cancel closes the wizard', async () => {
  server.use(createEnrollPairingSuccess());
  server.use(getCurrentEnrollPairingSuccess());
  const onClose = jest.fn();
  const user = userEvent.setup();

  render(makeWrapper({ close: onClose }));

  await screen.findByRole('img');
  await user.click(screen.getByRole('button', { name: /cancel/i }));
  await waitFor(() => expect(onClose).toHaveBeenCalled());
});
