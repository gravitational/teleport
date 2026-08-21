import { DeactivateDialog } from './DeactivateDialog';

export default {
  title: 'TeleportE/ClientIpRestrictions/DeactivateDialog',
  component: DeactivateDialog,
};

export const Default = () => (
  <DeactivateDialog
    onConfirm={() => {}}
    onCancel={() => {}}
    processing={false}
  />
);

export const Processing = () => (
  <DeactivateDialog
    onConfirm={() => {}}
    onCancel={() => {}}
    processing={true}
  />
);

export const WithError = () => (
  <DeactivateDialog
    onConfirm={() => {}}
    onCancel={() => {}}
    processing={false}
    error="failed to deactivate the IP allowlist"
  />
);
