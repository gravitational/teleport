import React from 'react'
import { storiesOf } from '@storybook/react'
import { ClusterLicense } from './License'

storiesOf('Gravity/License', module)
  .add('Valid License', () => {
    return (
      <ClusterLicense license={license} />
    );
  })
  .add('Expired License', () => {
    return (
      <ClusterLicense license={expiredLicense}/>
    );
  });

const license = {
  info: {
    expiration: new Date().toGMTString(),
    max_nodes: 100,
  },
  status: {
    isActive:  true,
    isError: false,
    message: 'error message'
  }
}

const expiredLicense = {
  ...license,
  status: {
    isError: true,
    message: 'License has expired'
  }
}
