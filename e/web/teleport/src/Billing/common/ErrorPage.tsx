import React from 'react';

import { ErrorPageProps } from 'e-teleport/Billing/types';

// todo (michellescripts) pending error page design as part of https://github.com/gravitational/cloud/issues/3536
export const ErrorPage = ({ message }: ErrorPageProps): React.ReactElement => (
  <h2>ERROR: {message}</h2>
);
