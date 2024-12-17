import { NewMfaDeviceForm } from 'teleport/components/NewMfaDeviceForm';
import { Attempt } from 'shared/hooks/useAttemptNext';
import { Auth2faType } from 'shared/services';

import useNewMfaDevice, { Props } from './useNewMfaDevice';

export default function Container(props: Props) {
  const state = useNewMfaDevice(props);
  return (
    <NewMfaDevice
      submitAttempt={state.attempt}
      clearSubmitAttempt={state.clearSubmitAttempt}
      qrCode={props.qrCode}
      auth2faType={state.auth2faType}
      credential={state.credential}
      createNewWebAuthnDevice={state.createNewWebAuthnDevice}
      onSubmitWithWebAuthn={state.setNewWebauthnDevice}
      onSubmit={state.setNewTotpDevice}
    />
  );
}

export interface NewMfaDeviceProps {
  submitAttempt: Attempt;
  clearSubmitAttempt: () => void;
  qrCode: string;
  auth2faType: Auth2faType;
  credential?: Credential;
  createNewWebAuthnDevice: () => void;
  onSubmitWithWebAuthn: (deviceName: string) => void;
  onSubmit: (otpCode: string, deviceName: string) => void;
  prev?: () => void;
}

export function NewMfaDevice(props: NewMfaDeviceProps) {
  return (
    <NewMfaDeviceForm
      title="Enroll New MFA Method"
      submitButtonText="Continue"
      shouldFocus={true}
      stepIndex={1}
      flowLength={4}
      {...props}
    />
  );
}
