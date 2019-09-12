import React from 'react';
import { storiesOf } from '@storybook/react';
import UpdateLicenseDialog from './UpdateLicenseDialog';

storiesOf('Gravity/License/UpdateLicenseDialog', module)
  .add('UpdateLicenseDialog', () => (
    <UpdateLicenseDialog {...dialogProps} />
  ))
  .add('Error', () => (
    <UpdateLicenseDialog { ...dialogProps } attempt={{isFailed: true, message: serverError}}
      />
  ));

const serverError = "this is a long error message which should be wrapped";

const dialogProps = {
  attempt: { },
  attemptActions: { },
  onUpdate: () => null,
  onClose: () => null
}
