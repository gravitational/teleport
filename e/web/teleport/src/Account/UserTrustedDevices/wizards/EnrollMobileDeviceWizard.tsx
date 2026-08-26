import { CloseButton } from '@gravitational/design-system';
import { useMutation, useQuery, useQueryClient } from '@tanstack/react-query';
import { useEffect, useState } from 'react';
import styled from 'styled-components';

import {
  Alert,
  Box,
  ButtonPrimary,
  ButtonSecondary,
  Flex,
  Image,
  Indicator,
  Link,
  P2,
  Stack,
  Subtitle1,
} from 'design';
import Dialog from 'design/Dialog';
import * as icons from 'design/Icon';
import { StepComponentProps, StepHeader, StepSlider } from 'design/StepSlider';
import { getErrorMessage } from 'shared/utils/error';

import { deviceService } from 'e-teleport/services/devices';
import { ApiError } from 'teleport/services/api/parseError';

import { enrollMobileDeviceWizardQueryKeyPrefix as queryKeyPrefix } from './const';

export const EnrollMobileDeviceWizard = (props: { close: () => void }) => {
  const queryClient = useQueryClient();
  // Bumping the key remounts the flow from the first step, which starts over with a fresh pairing.
  //
  // The removeQueries call isn't strictly necessary, but it makes the restart function correct by
  // inspection. It also makes sure old queries never leak if we forget to, say, set gcTime: 0 or
  // set a correct key.
  const [flowKey, setFlowKey] = useState(0);
  const restart = () => {
    queryClient.removeQueries({ queryKey: [queryKeyPrefix] });
    setFlowKey(key => key + 1);
  };

  return (
    <Dialog
      open
      disableEscapeKeyDown={false}
      dialogCss={() => ({ width: '650px' })}
      onClose={props.close}
    >
      <CloseButton
        position="absolute"
        top={3}
        insetEnd={3}
        type="button"
        aria-label="Close"
        onClick={props.close}
      />
      <StepSlider
        key={flowKey}
        flows={wizardFlows}
        currFlow="default"
        close={props.close}
        restart={restart}
      />
    </Dialog>
  );
};

const wizardFlows = {
  default: [QrCodeStep, WaitingForApprovalStep],
};

type EnrollMobileDeviceWizardStepProps = StepComponentProps & {
  /** Closes the wizard. */
  close: () => void;
  /** Starts the flow over from the first step with a fresh pairing. */
  restart: () => void;
};

// 228px is half of what the backend generates (456x456).
const qrCodeWidth = '228px';

function QrCodeStep(props: EnrollMobileDeviceWizardStepProps) {
  const createPairing = useMutation({
    mutationFn: deviceService.createEnrollPairing,
  });

  const { mutate: create } = createPairing;
  useEffect(() => {
    create();
  }, [create]);

  const pollPairing = useQuery({
    // Key on the pairing token so reopening the wizard doesn't reuse a cached
    // response from a previous pairing token.
    queryKey: [queryKeyPrefix, createPairing.data?.token],
    queryFn: deviceService.getCurrentEnrollPairing,
    // Enable the query only after the Create mutation resolves and only if it
    // returns an enroll pairing state that hasn't moved past awaiting_device,
    // since QrCodeStep displays EnrollPairing only in that state.
    enabled: createPairing.data?.state === 'awaiting_device',
    // The app-wide QueryClient already disables retries but this poll relies on that, so pin it
    // locally: a transient error recovers via refetchInterval refetching the errored query and a
    // 404 means the pairing is gone for good, where retries would only delay the denied-or-expired
    // state.
    retry: false,
    refetchInterval: query =>
      isNotFoundError(query.state.error) ? false : 3000,
  });

  // The pairing hitting its TTL before any device claims it, or a denial from another session, both
  // surface as a 404 from the poll.
  const expired = isNotFoundError(pollPairing.error);

  // Move to the next step if the enroll pairing has already transitioned past
  // awaiting_device state.
  const pairingState = (pollPairing.data || createPairing.data)?.state;
  const next = props.next;
  useEffect(() => {
    if (pairingState && pairingState !== 'awaiting_device') {
      next();
    }
  }, [pairingState, next]);

  if (expired) {
    return (
      <StepContainer ref={props.refCallback}>
        <Stack gap={4}>
          <StepHeader
            title="Request denied or expired"
            stepIndex={props.stepIndex}
            flowLength={props.flowLength}
            alignSelf="flex-start"
          />
          <ExpiredView restart={props.restart} close={props.close} />
        </Stack>
      </StepContainer>
    );
  }

  return (
    <StepContainer ref={props.refCallback}>
      <Stack gap={4} alignItems="center">
        <StepHeader
          title="Scan the QR code on your mobile device"
          stepIndex={props.stepIndex}
          flowLength={props.flowLength}
          alignSelf="flex-start"
        />
        {createPairing.error && (
          <Alert kind="danger" details={getErrorMessage(createPairing.error)}>
            Could not create an enrollment request
          </Alert>
        )}
        {pollPairing.error && (
          <Alert kind="danger" details={getErrorMessage(pollPairing.error)}>
            Could not poll the enrollment request
          </Alert>
        )}
        <Stack gap={2} alignItems="center">
          {(createPairing.isPending || createPairing.data?.qrCode) && (
            <Box
              borderRadius={4}
              borderWidth="1px"
              borderStyle="solid"
              borderColor="interactive.tonal.neutral.2"
              p={4}
              // With just "white" as the bg color, the spinner would be
              // invisible in the dark theme.
              backgroundColor={
                createPairing.isPending ? 'levels.surface' : 'white'
              }
            >
              {createPairing.isPending ? (
                <Flex
                  width={qrCodeWidth}
                  height={qrCodeWidth}
                  alignItems="center"
                  justifyContent="center"
                >
                  <Indicator delay="none" />
                </Flex>
              ) : (
                <Image
                  src={`data:image/png;base64,${createPairing.data.qrCode}`}
                  width={qrCodeWidth}
                  height={qrCodeWidth}
                />
              )}
            </Box>
          )}
          <Subtitle1 textAlign="center">
            Open the Teleport Verify app on your iPhone or iPad
            <br />
            and scan the QR code.
          </Subtitle1>
          <P2 textAlign="center" mt={0}>
            If you don&apos;t have the app on your device,
            <br />
            ask your administrator to{' '}
            {/* TODO(ravicious): Make sure this link stays up to date. PR with docs: https://github.com/gravitational/teleport/pull/64298 */}
            <Link
              href="https://goteleport.com/docs/identity-governance/device-trust/ios-admin-guide/"
              target="_blank"
            >
              set up Teleport Verify
            </Link>
            .
          </P2>
        </Stack>
        {/* TODO(ravicious): Make sure this link stays up to date. PR with docs: https://github.com/gravitational/teleport/pull/64298 */}
        <Flex width="100%" gap={2}>
          <ButtonSecondary
            size="large"
            as="a"
            href="https://goteleport.com/docs/identity-governance/device-trust/ios-user-guide/"
            target="_blank"
            block
          >
            Learn More <icons.NewTab ml={2} />
          </ButtonSecondary>
          <ButtonSecondary
            size="large"
            type="button"
            block
            onClick={props.close}
          >
            Cancel
          </ButtonSecondary>
        </Flex>
      </Stack>
    </StepContainer>
  );
}

/**
 * Shown once the mobile app has claimed the pairing. Approving unblocks the
 * CreatePairedDeviceEnrollToken call the app is waiting on, denying deletes the pairing. Approve is
 * an admin action.
 */
function WaitingForApprovalStep(props: EnrollMobileDeviceWizardStepProps) {
  const approve = useMutation({
    mutationFn: deviceService.approveEnrollPairing,
  });
  const deny = useMutation({ mutationFn: deviceService.denyEnrollPairing });
  // TODO(ravicious): Update the RFD number once the RFD is moved to the rfd repo.
  //
  // The pairing is deleted on TTL expiry and on denial, and no denied state is kept (RFD 32e), so a
  // 404 means it expired or was denied from another session, with no way to tell which.
  //
  // That holds for approve too: its 404 could also be a retry of an approval whose pairing the
  // mobile app already consumed. But the far likelier cause is the TTL lapsing during the MFA
  // ceremony.
  const expiredOnAction =
    isNotFoundError(approve.error) || isNotFoundError(deny.error);

  const pollPairing = useQuery({
    queryKey: [queryKeyPrefix, 'waiting-for-approval-poll'],
    queryFn: deviceService.getCurrentEnrollPairing,
    // TODO(ravicious): Update the RFD number once the RFD is moved to the rfd repo.
    //
    // The first fetch brings the pairing and with it the device asking to enroll. Later fetches
    // notice the pairing expiring (due to its TTL or denial) or being approved from another
    // session, like the move-to-desktop flow from RFD 32e.
    //
    // A local approval or denial does not disable the poll. It stops on its own instead: the
    // resolved pairing is gone from the backend, so the next tick 404s and refetchInterval turns
    // that into a stop. The state check covers the poll observing an approval, including one from
    // another session.
    enabled: query => query.state.data?.state !== 'approved',
    refetchInterval: query =>
      isNotFoundError(query.state.error) ? false : 3000,
    // The key is per user, not per pairing, so drop the cache as soon as the step unmounts. A
    // reopened wizard must not see the previous pairing's resolved state, and with the poll stopped
    // once the pairing is resolved a refetch would never correct it.
    gcTime: 0,
    // The app-wide QueryClient already disables retries, but this poll relies on that, so pin it
    // locally. A transient error recovers on its own because refetchInterval keeps refetching an
    // errored query, and a 404 means the pairing is gone for good, so retries would only delay the
    // denied-or-expired-state.
    retry: false,
  });

  const pairing = pollPairing.data;
  const approved = approve.isSuccess || pairing?.state === 'approved';
  // A deny can succeed on a pairing that another session already approved, if it lands before the
  // mobile app consumes the pairing. The approval won on the backend, so it wins the display too.
  const denied = deny.isSuccess && !approved;
  const resolved = approved || denied;
  // A pairing resolved here is gone from the backend before the poll winds down: an approved
  // pairing is consumed by the mobile app and a denied one is deleted by the deny, so one last
  // poll tick can return a 404. That 404 must not flip the step to the expired state: the pairing
  // was approved or denied, not expired.
  const expired =
    !resolved && (expiredOnAction || isNotFoundError(pollPairing.error));
  const decisionPending = approve.isPending || deny.isPending;

  let title = 'Approve the request';
  if (expired) {
    title = 'Request denied or expired';
  } else if (approved) {
    title = 'Request approved';
  } else if (denied) {
    title = 'Request denied';
  }

  return (
    <StepContainer ref={props.refCallback}>
      <Stack gap={4}>
        <StepHeader
          title={title}
          stepIndex={props.stepIndex}
          flowLength={props.flowLength}
          alignSelf="flex-start"
        />

        {expired && <ExpiredView restart={props.restart} close={props.close} />}

        {!expired && (
          <>
            {/* Any errors here are non-404 by construction: a 404 from the poll or approve/deny
              flips the step to the expired state. A resolved pairing hides the errors, because a
              stale error can outlive the resolution: an approve whose response was lost still lands
              and the next poll tick observes the pairing approved, or another session approves
              while a poll tick or a deny here fails. Without the guard the step would report the
              same pairing as both approved and failed. */}
            {!resolved && pollPairing.error && (
              <Alert kind="danger" details={getErrorMessage(pollPairing.error)}>
                Could not poll the enrollment request
              </Alert>
            )}
            {!resolved && approve.error && (
              <Alert kind="danger" details={getErrorMessage(approve.error)}>
                Could not approve the enrollment request
              </Alert>
            )}

            {!resolved && deny.error && (
              <Alert kind="danger" details={getErrorMessage(deny.error)}>
                Could not deny the enrollment request
              </Alert>
            )}

            {approved && (
              <P2>
                Finish enrolling the device in Teleport&nbsp;Verify on your
                mobile device.
              </P2>
            )}

            {denied && <P2>The device was not enrolled.</P2>}

            {resolved ? (
              <ButtonSecondary
                size="large"
                type="button"
                block
                onClick={props.close}
              >
                Close
              </ButtonSecondary>
            ) : (
              <>
                <P2>
                  {pairing?.device
                    ? `Do you want to enroll the ${pairing.device.osType} ${pairing.device.osVersion} device with serial number ${pairing.device.serialNumber}?`
                    : 'Your mobile device has requested enrollment.'}
                </P2>
                <Flex width="100%" gap={2}>
                  <ButtonPrimary
                    size="large"
                    type="button"
                    block
                    disabled={!pairing || decisionPending}
                    onClick={() => approve.mutate(pairing.token)}
                  >
                    Approve
                  </ButtonPrimary>
                  <ButtonSecondary
                    size="large"
                    type="button"
                    block
                    disabled={!pairing || decisionPending}
                    onClick={() => deny.mutate(pairing.token)}
                  >
                    Deny
                  </ButtonSecondary>
                </Flex>
              </>
            )}
          </>
        )}
      </Stack>
    </StepContainer>
  );
}

/**
 * ExpiredView is the terminal state for a pairing that is gone without this session resolving it.
 * The combined wording stays accurate on both steps: denial from another session and TTL expiry are
 * indistinguishable here.
 */
function ExpiredView(props: { restart: () => void; close: () => void }) {
  return (
    <Stack gap={4} width="100%">
      <P2 m={0}>
        The enrollment request was denied or has expired. Start over and scan a
        new QR code with your mobile device.
      </P2>
      <Flex width="100%" gap={2}>
        <ButtonPrimary size="large" type="button" block onClick={props.restart}>
          Start Over
        </ButtonPrimary>
        <ButtonSecondary size="large" type="button" block onClick={props.close}>
          Close
        </ButtonSecondary>
      </Flex>
    </Stack>
  );
}

/**
 * Sets the padding on the dialog content instead of the dialog itself to make
 * the slide animations reach the dialog border.
 */
const StepContainer = styled.div`
  padding: ${props => props.theme.space[5]}px;
  padding-top: ${props => props.theme.space[4]}px;
`;

function isNotFoundError(err: unknown): boolean {
  return err instanceof ApiError && err.response.status === 404;
}
