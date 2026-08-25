import { userEvent } from '@testing-library/user-event';
import { http, HttpResponse } from 'msw';
import { ComponentProps } from 'react';

import {
  enableMswServer,
  render,
  screen,
  server,
  testQueryClient,
  waitFor,
} from 'design/utils/testing';

import cfg from 'e-teleport/config';
import { createTeleportContextE } from 'e-teleport/mocks/contexts';
import { GetEnrollPairingResponse } from 'e-teleport/services/devices';
import TeleportContextE from 'e-teleport/teleportContextE';
import {
  approveEnrollPairingError,
  approveEnrollPairingSuccess,
  createEnrollPairingError,
  createEnrollPairingSuccess,
  denyEnrollPairingError,
  denyEnrollPairingSuccess,
  getCurrentEnrollPairingError,
  getCurrentEnrollPairingSuccess,
} from 'e-teleport/test/helpers/enrollPairing';
import TeleportContextProvider from 'teleport/TeleportContextProvider';

import { enrollMobileDeviceWizardQueryKeyPrefix as queryKeyPrefix } from './const';
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

describe('QrCodeStep', () => {
  test('renders the QR code once the pairing is created', async () => {
    server.use(createEnrollPairingSuccess());
    server.use(getCurrentEnrollPairingSuccess());

    render(makeWrapper());

    const image = await screen.findByRole('img');
    expect(image).toHaveAttribute(
      'src',
      'data:image/png;base64,base64-qr-code'
    );
    expect(
      screen.getByText(/Scan the QR code on your mobile device/i)
    ).toBeInTheDocument();
  });

  test('surfaces a createEnrollPairing error', async () => {
    server.use(createEnrollPairingError(500, 'boom'));

    render(makeWrapper());

    expect(
      await screen.findByText('Could not create an enrollment request')
    ).toBeInTheDocument();
    expect(screen.getByText('boom')).toBeInTheDocument();
  });

  test('surfaces a getCurrentEnrollPairing error', async () => {
    server.use(createEnrollPairingSuccess());
    server.use(getCurrentEnrollPairingError(500, 'poll boom'));

    render(makeWrapper());

    expect(
      await screen.findByText('Could not poll the enrollment request')
    ).toBeInTheDocument();
    expect(screen.getByText('poll boom')).toBeInTheDocument();
  });

  test('shows the denied-or-expired state when the pairing expires before it is claimed', async () => {
    server.use(createEnrollPairingSuccess());
    server.use(getCurrentEnrollPairingError(404, 'enroll pairing not found'));

    render(makeWrapper());

    expect(
      await screen.findByText('Request denied or expired')
    ).toBeInTheDocument();
    expect(
      screen.getByRole('button', { name: 'Start Over' })
    ).toBeInTheDocument();
    // Verify that the stale QR code and the generic poll error are replaced, not overlaid.
    expect(screen.queryByRole('img')).not.toBeInTheDocument();
    expect(
      screen.queryByText('Could not poll the enrollment request')
    ).not.toBeInTheDocument();
  });

  test('start over from an expired QR step restarts with a fresh pairing', async () => {
    let createCalls = 0;
    server.use(
      http.post(cfg.api.enrollPairing, () => {
        createCalls += 1;
        // First a pairing that expires before it is claimed, then a fresh one.
        if (createCalls === 1) {
          return HttpResponse.json({
            state: 'awaiting_device',
            token: 'pairing-token',
            qrCode: 'base64-qr-code',
          });
        }
        return HttpResponse.json({
          state: 'awaiting_device',
          token: 'fresh-pairing-token',
          qrCode: 'fresh-base64-qr-code',
        });
      })
    );

    // The first pairing is gone, the fresh one polls successfully so the wizard stays on the QR
    // step.
    server.use(
      http.get(cfg.api.enrollPairing, () => {
        if (createCalls === 1) {
          return HttpResponse.json(
            { error: { message: 'enroll pairing not found' } },
            { status: 404 }
          );
        }
        return HttpResponse.json({
          state: 'awaiting_device',
          token: 'fresh-pairing-token',
        });
      })
    );
    const user = userEvent.setup();

    render(makeWrapper());

    await user.click(await screen.findByRole('button', { name: 'Start Over' }));

    const image = await screen.findByRole('img');
    expect(image).toHaveAttribute(
      'src',
      'data:image/png;base64,fresh-base64-qr-code'
    );
    expect(createCalls).toBe(2);
  });

  test('advances past the QR step when createEnrollPairing returns a pairing past awaiting_device', async () => {
    // createEnrollPairing returned an existing pairing already in a later state,
    // e.g. the user opened the wizard before, the iOS app already pinged the
    // public RPC, and the user re-opened the wizard to approve. The QR step never
    // enables polling and the wizard advances on the createEnrollPairing result alone.
    server.use(
      createEnrollPairingSuccess({
        state: 'awaiting_approval',
        token: 'pairing-token',
      })
    );
    // Consumed by the approval step's poll after the advance, not by the QR step, whose poll stays
    // disabled for a pairing past awaiting_device.
    server.use(
      getCurrentEnrollPairingSuccess({
        state: 'awaiting_approval',
        token: 'pairing-token',
      })
    );

    render(makeWrapper());

    // StepSlider keeps both steps in the DOM during the slide animation, so
    // only assert the new step appears rather than that the old one is gone.
    expect(await screen.findByText('Approve the request')).toBeInTheDocument();
  });

  test('advances past the QR step when getCurrentEnrollPairing returns a pairing past awaiting_device', async () => {
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

    expect(await screen.findByText('Approve the request')).toBeInTheDocument();
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
});

describe('WaitingForApprovalStep', () => {
  // awaitingApproval is what the wizard sees right after the mobile app claims the pairing. When
  // createEnrollPairing returns it, the wizard skips the QR step. It also carries the persisted
  // device, so the getCurrentEnrollPairing poll lets the approval step show which device is asking
  // to enroll.
  const awaitingApproval = {
    state: 'awaiting_approval',
    token: 'pairing-token',
    device: {
      osType: 'iOS',
      serialNumber: 'CXXXXXXXXX01',
      osVersion: '26.3.1',
    },
  } satisfies GetEnrollPairingResponse;

  const deviceQuestion =
    'Do you want to enroll the iOS 26.3.1 device with serial number CXXXXXXXXX01?';

  test('shows which device is asking to enroll', async () => {
    server.use(createEnrollPairingSuccess(awaitingApproval));
    server.use(getCurrentEnrollPairingSuccess(awaitingApproval));

    render(makeWrapper());

    expect(await screen.findByText(deviceQuestion)).toBeInTheDocument();
  });

  test('surfaces pollPairing error', async () => {
    server.use(createEnrollPairingSuccess(awaitingApproval));
    server.use(getCurrentEnrollPairingError(500, 'poll boom'));

    render(makeWrapper());

    expect(
      await screen.findByText('Could not poll the enrollment request')
    ).toBeInTheDocument();
    expect(screen.getByText('poll boom')).toBeInTheDocument();
  });

  test('approving tells the user to finish enrolling on the device', async () => {
    server.use(createEnrollPairingSuccess(awaitingApproval));
    server.use(getCurrentEnrollPairingSuccess(awaitingApproval));
    server.use(approveEnrollPairingSuccess());
    const user = userEvent.setup();

    render(makeWrapper());

    // The buttons are disabled until the poll returns the pairing, so wait for the device question
    // before clicking.
    await screen.findByText(deviceQuestion);
    await user.click(screen.getByRole('button', { name: 'Approve' }));

    expect(await screen.findByText('Request approved')).toBeInTheDocument();
    expect(
      screen.getByText(/Finish enrolling the device in Teleport Verify/)
    ).toBeInTheDocument();
    expect(
      screen.queryByRole('button', { name: 'Approve' })
    ).not.toBeInTheDocument();
  });

  test('surfaces an approve error', async () => {
    server.use(createEnrollPairingSuccess(awaitingApproval));
    server.use(getCurrentEnrollPairingSuccess(awaitingApproval));
    server.use(approveEnrollPairingError(500, 'approve error'));
    const user = userEvent.setup();

    render(makeWrapper());

    await screen.findByText(deviceQuestion);
    await user.click(screen.getByRole('button', { name: 'Approve' }));

    expect(
      await screen.findByText('Could not approve the enrollment request')
    ).toBeInTheDocument();
    expect(screen.getByText('approve error')).toBeInTheDocument();
    // Verify that the user can try approving again.
    expect(screen.getByRole('button', { name: 'Approve' })).toBeInTheDocument();
  });

  test('denying reports that the device was not enrolled', async () => {
    server.use(createEnrollPairingSuccess(awaitingApproval));
    server.use(getCurrentEnrollPairingSuccess(awaitingApproval));
    server.use(denyEnrollPairingSuccess());
    const user = userEvent.setup();

    render(makeWrapper());

    await screen.findByText(deviceQuestion);
    await user.click(screen.getByRole('button', { name: 'Deny' }));

    expect(await screen.findByText('Request denied')).toBeInTheDocument();
    expect(
      screen.getByText('The device was not enrolled.')
    ).toBeInTheDocument();
  });

  test('keeps the denied state once the poll observes the pairing deleted by the deny', async () => {
    server.use(createEnrollPairingSuccess(awaitingApproval));
    let denyCalled = false;
    server.use(
      http.post(cfg.api.enrollPairingDeny, () => {
        denyCalled = true;
        return HttpResponse.json({ message: 'ok' });
      })
    );
    // The deny deletes the pairing, so a poll tick that lands after it returns the 404 left by the
    // deletion. That 404 must not flip the step to the expired state: the pairing was denied here.
    server.use(
      http.get(cfg.api.enrollPairing, () => {
        if (denyCalled) {
          return HttpResponse.json(
            { error: { message: 'enroll pairing not found' } },
            { status: 404 }
          );
        }
        return HttpResponse.json(awaitingApproval);
      })
    );
    const user = userEvent.setup();

    render(makeWrapper());

    await screen.findByText(deviceQuestion);
    await user.click(screen.getByRole('button', { name: 'Deny' }));
    await screen.findByText('Request denied');

    // Skip waiting for the 3-second refetchInterval tick.
    await testQueryClient.refetchQueries({ queryKey: [queryKeyPrefix] });
    // The awaited refetch leaves the 404 in the query cache but does not flush the re-render it
    // triggers, so let waitFor run a render pass before asserting on the DOM.
    await waitFor(() =>
      expect(
        testQueryClient.getQueryState([
          queryKeyPrefix,
          'waiting-for-approval-poll',
        ])?.error
      ).toBeDefined()
    );

    expect(screen.getByText('Request denied')).toBeInTheDocument();
    expect(
      screen.getByText('The device was not enrolled.')
    ).toBeInTheDocument();
    expect(
      screen.queryByText('Request denied or expired')
    ).not.toBeInTheDocument();
  });

  test('surfaces a deny error', async () => {
    server.use(createEnrollPairingSuccess(awaitingApproval));
    server.use(getCurrentEnrollPairingSuccess(awaitingApproval));
    server.use(denyEnrollPairingError(500, 'deny error'));
    const user = userEvent.setup();

    render(makeWrapper());

    await screen.findByText(deviceQuestion);
    await user.click(screen.getByRole('button', { name: 'Deny' }));

    expect(
      await screen.findByText('Could not deny the enrollment request')
    ).toBeInTheDocument();
    expect(screen.getByText('deny error')).toBeInTheDocument();
    // Verify that the user can try denying again.
    expect(screen.getByRole('button', { name: 'Deny' })).toBeInTheDocument();
  });

  test('shows the denied-or-expired state when deny finds the pairing already gone', async () => {
    server.use(createEnrollPairingSuccess(awaitingApproval));
    server.use(getCurrentEnrollPairingSuccess(awaitingApproval));
    server.use(denyEnrollPairingError(404, 'enroll pairing not found'));
    const user = userEvent.setup();

    render(makeWrapper());

    await screen.findByText(deviceQuestion);
    await user.click(screen.getByRole('button', { name: 'Deny' }));

    expect(
      await screen.findByText('Request denied or expired')
    ).toBeInTheDocument();
    expect(
      screen.getByRole('button', { name: 'Start Over' })
    ).toBeInTheDocument();
    expect(
      screen.queryByRole('button', { name: 'Approve' })
    ).not.toBeInTheDocument();
  });

  test('hides the deny error once the user approves instead', async () => {
    server.use(createEnrollPairingSuccess(awaitingApproval));
    server.use(getCurrentEnrollPairingSuccess(awaitingApproval));
    server.use(denyEnrollPairingError(500, 'deny error'));
    server.use(approveEnrollPairingSuccess());
    const user = userEvent.setup();

    render(makeWrapper());

    await screen.findByText(deviceQuestion);
    await user.click(screen.getByRole('button', { name: 'Deny' }));
    await screen.findByText('Could not deny the enrollment request');

    // A failed deny leaves the buttons enabled, so the user can approve instead. The stale deny
    // error must not be reported alongside the approved state.
    await user.click(screen.getByRole('button', { name: 'Approve' }));

    expect(await screen.findByText('Request approved')).toBeInTheDocument();
    expect(
      screen.queryByText('Could not deny the enrollment request')
    ).not.toBeInTheDocument();
  });

  test('shows the denied-or-expired state when the pairing disappears mid-approval', async () => {
    server.use(
      createEnrollPairingSuccess({
        state: 'awaiting_approval',
        token: 'pairing-token',
      })
    );
    server.use(getCurrentEnrollPairingError(404, 'enroll pairing not found'));

    render(makeWrapper());

    expect(
      await screen.findByText('Request denied or expired')
    ).toBeInTheDocument();
    expect(
      screen.getByRole('button', { name: 'Start Over' })
    ).toBeInTheDocument();
    expect(
      screen.queryByRole('button', { name: 'Approve' })
    ).not.toBeInTheDocument();
  });

  test('treats a 404 on approve as denial or expiry', async () => {
    server.use(createEnrollPairingSuccess(awaitingApproval));
    server.use(getCurrentEnrollPairingSuccess(awaitingApproval));
    // The pairing hit its TTL between the poll observing it and the approve landing, e.g. while the
    // MFA ceremony ran.
    server.use(approveEnrollPairingError(404, 'enroll pairing not found'));
    const user = userEvent.setup();

    render(makeWrapper());

    await screen.findByText(deviceQuestion);
    await user.click(screen.getByRole('button', { name: 'Approve' }));

    expect(
      await screen.findByText('Request denied or expired')
    ).toBeInTheDocument();
    expect(
      screen.getByRole('button', { name: 'Start Over' })
    ).toBeInTheDocument();
    expect(screen.queryByText('Request approved')).not.toBeInTheDocument();
    expect(
      screen.queryByText('Could not approve the enrollment request')
    ).not.toBeInTheDocument();
  });

  test('shows the approved state when the pairing was approved from another session', async () => {
    // RFD 32e supports starting the flow in a browser on the mobile device and approving from a
    // desktop session. The poll of the first session then observes the approved state without any
    // local mutation.
    server.use(createEnrollPairingSuccess(awaitingApproval));
    server.use(
      getCurrentEnrollPairingSuccess({ ...awaitingApproval, state: 'approved' })
    );

    render(makeWrapper());

    expect(await screen.findByText('Request approved')).toBeInTheDocument();
    expect(
      screen.getByText(/Finish enrolling the device in Teleport Verify/)
    ).toBeInTheDocument();
    expect(
      screen.queryByRole('button', { name: 'Approve' })
    ).not.toBeInTheDocument();
  });

  test('approval from another session wins over a deny still in flight', async () => {
    server.use(createEnrollPairingSuccess(awaitingApproval));
    let approvedElsewhere = false;
    server.use(
      http.get(cfg.api.enrollPairing, () => {
        if (approvedElsewhere) {
          return HttpResponse.json({ ...awaitingApproval, state: 'approved' });
        }
        return HttpResponse.json(awaitingApproval);
      })
    );
    // The deny response is held back until the poll observes the approval from the other session,
    // so deny.isSuccess lands while the pairing already shows as approved. The step must not report
    // the pairing as both approved and denied.
    const denyGate = Promise.withResolvers<void>();
    server.use(
      http.post(cfg.api.enrollPairingDeny, async () => {
        await denyGate.promise;
        return HttpResponse.json({ message: 'ok' });
      })
    );
    const user = userEvent.setup();

    render(makeWrapper());

    await screen.findByText(deviceQuestion);
    await user.click(screen.getByRole('button', { name: 'Deny' }));

    approvedElsewhere = true;
    // Skip waiting for the 3-second refetchInterval tick.
    await testQueryClient.refetchQueries({ queryKey: [queryKeyPrefix] });
    await screen.findByText('Request approved');

    // Grab the deny mutation while the gate still holds it pending. It is the only in-flight
    // mutation at this point, createPairing settled when the wizard rendered.
    const [denyMutation] = testQueryClient
      .getMutationCache()
      .findAll({ status: 'pending' });

    denyGate.resolve();

    // Wait for the deny to settle as success rather than waiting out a findByText timeout on text
    // that must never appear. waitFor runs a render pass, so the asserts below observe the DOM
    // after deny.isSuccess landed.
    await waitFor(() => expect(denyMutation.state.status).toBe('success'));

    expect(
      screen.queryByText('The device was not enrolled.')
    ).not.toBeInTheDocument();
    expect(screen.getByText('Request approved')).toBeInTheDocument();
    expect(
      screen.getByText(/Finish enrolling the device in Teleport Verify/)
    ).toBeInTheDocument();
  });

  test('hides the approve error once the poll observes the pairing approved', async () => {
    server.use(createEnrollPairingSuccess(awaitingApproval));
    // The approve lands on the backend but its response is lost, or another session approves while
    // this one fails. Either way the pairing resolves while approve.error is still set, and the
    // step must not report the pairing as both approved and failed.
    server.use(approveEnrollPairingError(500, 'approve error'));
    let pollCalls = 0;
    server.use(
      http.get(cfg.api.enrollPairing, () => {
        pollCalls += 1;
        if (pollCalls === 1) {
          return HttpResponse.json(awaitingApproval);
        }
        return HttpResponse.json({ ...awaitingApproval, state: 'approved' });
      })
    );
    const user = userEvent.setup();

    render(makeWrapper());

    await screen.findByText(deviceQuestion);
    await user.click(screen.getByRole('button', { name: 'Approve' }));
    await screen.findByText('Could not approve the enrollment request');

    // Skip waiting for the 3-second refetchInterval tick.
    await testQueryClient.refetchQueries({ queryKey: [queryKeyPrefix] });

    expect(await screen.findByText('Request approved')).toBeInTheDocument();
    expect(
      screen.queryByText('Could not approve the enrollment request')
    ).not.toBeInTheDocument();
  });

  test('a reopened wizard does not reuse the approved state of a previous pairing', async () => {
    server.use(createEnrollPairingSuccess(awaitingApproval));
    server.use(
      getCurrentEnrollPairingSuccess({ ...awaitingApproval, state: 'approved' })
    );

    const { unmount } = render(makeWrapper());
    expect(await screen.findByText('Request approved')).toBeInTheDocument();

    // Close the wizard, then reopen it with a fresh pairing awaiting approval. No wait is needed in
    // between, as the cache eviction is scheduled on unmount (gcTime 0) always fires before the
    // reopened wizard can finish createEnrollPairing and mount its own approval step.
    unmount();
    server.use(getCurrentEnrollPairingSuccess(awaitingApproval));

    render(makeWrapper());

    expect(await screen.findByText(deviceQuestion)).toBeInTheDocument();
  });

  test('start over from an expired approval step restarts with a fresh pairing', async () => {
    let createCalls = 0;
    server.use(
      http.post(cfg.api.enrollPairing, () => {
        createCalls += 1;
        // First a pairing that expires mid-approval, then a fresh one that puts the wizard
        // back on the QR step.
        if (createCalls === 1) {
          return HttpResponse.json({
            state: 'awaiting_approval',
            token: 'pairing-token',
          });
        }
        return HttpResponse.json({
          state: 'awaiting_device',
          token: 'fresh-pairing-token',
          qrCode: 'base64-qr-code',
        });
      })
    );
    // The first pairing is gone, the fresh one polls successfully so the wizard stays on the QR step.
    server.use(
      http.get(cfg.api.enrollPairing, () => {
        if (createCalls === 1) {
          return HttpResponse.json(
            { error: { message: 'enroll pairing not found' } },
            { status: 404 }
          );
        }
        return HttpResponse.json({
          state: 'awaiting_device',
          token: 'fresh-pairing-token',
        });
      })
    );
    const user = userEvent.setup();

    render(makeWrapper());

    await user.click(await screen.findByRole('button', { name: 'Start Over' }));

    const image = await screen.findByRole('img');
    expect(image).toHaveAttribute(
      'src',
      'data:image/png;base64,base64-qr-code'
    );
    expect(createCalls).toBe(2);
  });
});

describe('EnrollMobileDeviceWizard', () => {
  test('the corner close button closes the wizard', async () => {
    server.use(createEnrollPairingSuccess());
    server.use(getCurrentEnrollPairingSuccess());
    const onClose = jest.fn();
    const user = userEvent.setup();

    render(makeWrapper({ close: onClose }));

    await user.click(await screen.findByRole('button', { name: 'Close' }));
    await waitFor(() => expect(onClose).toHaveBeenCalled());
  });
});
