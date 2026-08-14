import { useMutation, useQuery } from '@tanstack/react-query';
import { useEffect } from 'react';
import styled from 'styled-components';

import {
  Alert,
  Box,
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

export const EnrollMobileDeviceWizard = (props: { close: () => void }) => {
  return (
    <Dialog
      open
      disableEscapeKeyDown={false}
      dialogCss={() => ({ width: '650px' })}
      onClose={props.close}
    >
      <StepSlider flows={wizardFlows} currFlow="default" close={props.close} />
    </Dialog>
  );
};

const wizardFlows = {
  default: [QrCodeStep, WaitingForApprovalStep],
};

type EnrollMobileDeviceWizardStepProps = StepComponentProps & {
  /** Closes the wizard. */
  close: () => void;
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
    queryKey: ['enrollPairing', createPairing.data?.token],
    queryFn: deviceService.getCurrentEnrollPairing,
    // Enable the query only after the Create mutation resolves and only if it
    // returns an enroll pairing state that hasn't moved past awaiting_device,
    // since QrCodeStep displays EnrollPairing only in that state.
    enabled: createPairing.data?.state === 'awaiting_device',
    refetchInterval: 3000,
  });

  // Move to the next step if the enroll pairing has already transitioned past
  // awaiting_device state.
  const pairingState = (pollPairing.data || createPairing.data)?.state;
  const next = props.next;
  useEffect(() => {
    if (pairingState && pairingState !== 'awaiting_device') {
      next();
    }
  }, [pairingState, next]);

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
            Could not create an enrollment pairing
          </Alert>
        )}
        {pollPairing.error && (
          <Alert kind="danger" details={getErrorMessage(pollPairing.error)}>
            Could not poll the enrollment pairing
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
            Open the Camera app on your iPhone or iPad
            <br />
            and scan the QR code to open Teleport&nbsp;Verify.
          </Subtitle1>
          <P2 textAlign="center" mt={0}>
            If you see &quot;No usable data found&quot; when scanning the code,
            <br />
            ask your administrator to{' '}
            {/* TODO(ravicious): Make sure this link stays up to date. PR with docs: https://github.com/gravitational/teleport/pull/64298 */}
            <Link
              href="https://goteleport.com/docs/identity-governance/device-trust/ios-admin-guide/"
              target="_blank"
            >
              set up Teleport Verify
            </Link>{' '}
            on your mobile device.
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

// Placeholder shown after the iOS app contacts the public RPC. The approval
// step (MFA-gated approve/deny) lands in a follow-up issue. Until then we just
// acknowledge the request and close.
function WaitingForApprovalStep(props: EnrollMobileDeviceWizardStepProps) {
  return (
    <StepContainer ref={props.refCallback}>
      <Stack gap={4}>
        <StepHeader
          title="Request received"
          stepIndex={props.stepIndex}
          flowLength={props.flowLength}
          alignSelf="flex-start"
        />
        <P2>Your mobile device has requested enrollment.</P2>
        <ButtonSecondary size="large" type="button" block onClick={props.close}>
          Close
        </ButtonSecondary>
      </Stack>
    </StepContainer>
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
