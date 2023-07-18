import React from 'react';

import { ResourceIcon, iconNames } from 'design/ResourceIcon';
import { Flex } from 'design';

export default {
  title: 'Design/ResourceIcon',
};

export const Icons = () => {
  return (
    <>
      {iconNames.map(name => (
        <Flex gap={3} alignItems="center">
          <ResourceIcon name={name} width="100px" />{' '}
          <ResourceIcon name={name} width="25px" />
          {name}
        </Flex>
      ))}
    </>
  );
};
