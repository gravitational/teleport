import { Meta, StoryObj } from '@storybook/react-vite';

import { TeleportProviderBasicE } from 'e-teleport/mocks/providers';
import {
  approveEnrollPairingSuccess,
  createEnrollPairingError,
  createEnrollPairingForever,
  createEnrollPairingSuccess,
  denyEnrollPairingSuccess,
  getCurrentEnrollPairingError,
  getCurrentEnrollPairingSuccess,
} from 'e-teleport/test/helpers/enrollPairing';

import { EnrollMobileDeviceWizard } from './EnrollMobileDeviceWizard';

const meta: Meta<typeof Wrapper> = {
  title: 'TeleportE/Account/UserTrustedDevices/EnrollMobileDeviceWizard',
  component: Wrapper,
};
export default meta;

type Story = StoryObj<typeof meta>;

const dummyQrCode =
  'iVBORw0KGgoAAAANSUhEUgAAAcgAAAHIAQAAAADi2kdHAAADCklEQVR4AWL4Ty4YEJ0PGKCg/gH7PwgBaO8Ocly3gSAMc1Y+ho+aHNXH0CqViK5ikTYwyPYBPxfGSO5PqwbZakqe8bx+pNf4kaScG/eh/7rHIxKJfMgBum4+4/+Tfz/nh7Ti78MZN89VIpH3aSee5auH77RUrqaeOyUS6ewbT6fgkXNXMnI8fpNIpHRHzSVO0h2wp2UPK5HIroL57nm1fsoC6HkrX5zrJxJ5VuP/48MjEolURlKrd2+exjb0VIclEvl6JP65KqlMaK6kvCj6Bs8l+QOJtNTVTOtcplV5txBX6vJDIpEbyl9OwdWiDHcKen6LRCInegfM+PldpC/Z4lxrIJGWs69t1Kh/LKWPuWysayCRlqmQ8uFGdvoDnbcaNysuIZGWur53a101BeUa69wMRiIrHTpy35/sS9W0r4dKMiKRkd2UTab5GmMGOCPb4U6ckMjKc0+/65x6M9dpLMmY7EMiHf/m/suhKyO15q02K5HIyH7XXvfYHoOcfLS6CkciI/cdkXPH/2x4T+kzMwSJjLw6KWmiyXOuq6ASkmociazUR0m+NwlkOY7Ha8eSSGR2P7oKhjfTRvtJ3fu3RCKPtpEnqnQsu2kS1FUQiYxsVkmplc4W5Uw3j233zRKJnGtao77bS5nB0qx0mxuJjMwmfk+nksoNXvJwBJ0SiZTy0FEak9nd3yt0+eKZ6Z6yRCLbFXAeNtNyoU5t+8PaSGRl+knn2pdutie0rzdkLZHI11F+//XxgojPtT/QJrglEqlVK421znXftmukz+l8lwQpIVsh5f2ivkHUj3ybVNWSSKR0NLd1LIVdALV1D5K3SOT5PLXywEhCe1vXzf7+lhoSGdm3YZXSaR+Pbrz1i3loiURqje7Rqp3I0aXw/AUIJLL39h7HzdzxhIhlQpDIyvN30Lofu1rak3fs/QEksnLVT6ma0phMfC50PkX7LZHIY1P2+A3QiT46UEjkKdvczug7jR3HKohERv7yA3yjO23b4VsgkZFd7JyHOcw5f6S5nTy0RCL/qP9Q8y+uqV5+LO09aQAAAABJRU5ErkJggg==';

export const QrCodeStep: Story = {
  beforeEach({ msw }) {
    msw.use(
      createEnrollPairingSuccess({
        state: 'awaiting_device',
        token: 'pairing-token',
        qrCode: dummyQrCode,
      }),
      getCurrentEnrollPairingSuccess()
    );
  },
};

export const QrCodeStepCreatePending: Story = {
  beforeEach({ msw }) {
    msw.use(createEnrollPairingForever());
  },
};

export const QrCodeStepCreateError: Story = {
  beforeEach({ msw }) {
    msw.use(createEnrollPairingError(500, 'something went wrong'));
  },
};

export const QrCodeStepPollError: Story = {
  beforeEach({ msw }) {
    msw.use(
      createEnrollPairingSuccess({
        state: 'awaiting_device',
        token: 'pairing-token',
        qrCode: dummyQrCode,
      }),
      getCurrentEnrollPairingError(500, 'something went wrong')
    );
  },
};

export const QrCodeStepExpired: Story = {
  beforeEach({ msw }) {
    msw.use(
      createEnrollPairingSuccess({
        state: 'awaiting_device',
        token: 'pairing-token',
        qrCode: dummyQrCode,
      }),
      getCurrentEnrollPairingError(404, 'enroll pairing not found')
    );
  },
};

export const WaitingForApprovalStep: Story = {
  beforeEach({ msw }) {
    msw.use(
      // Create returns a pairing already past awaiting_device,
      // so the wizard skips QrCodeStep on mount.
      createEnrollPairingSuccess({
        state: 'awaiting_approval',
        token: 'pairing-token',
      }),
      getCurrentEnrollPairingSuccess({
        state: 'awaiting_approval',
        token: 'pairing-token',
        device: {
          osType: 'iOS',
          serialNumber: 'CXXXXXXXXX01',
          osVersion: '26.3.1',
        },
      }),
      approveEnrollPairingSuccess(),
      denyEnrollPairingSuccess()
    );
  },
};

export const WaitingForApprovalStepApproved: Story = {
  beforeEach({ msw }) {
    msw.use(
      createEnrollPairingSuccess({
        state: 'awaiting_approval',
        token: 'pairing-token',
      }),
      getCurrentEnrollPairingSuccess({
        state: 'approved',
        token: 'pairing-token',
        device: {
          osType: 'iOS',
          serialNumber: 'CXXXXXXXXX01',
          osVersion: '26.3.1',
        },
      })
    );
  },
};

export const WaitingForApprovalStepExpired: Story = {
  beforeEach({ msw }) {
    msw.use(
      createEnrollPairingSuccess({
        state: 'awaiting_approval',
        token: 'pairing-token',
      }),
      getCurrentEnrollPairingError(404, 'enroll pairing not found')
    );
  },
};

function Wrapper() {
  return (
    <TeleportProviderBasicE>
      <EnrollMobileDeviceWizard close={() => {}} />
    </TeleportProviderBasicE>
  );
}
