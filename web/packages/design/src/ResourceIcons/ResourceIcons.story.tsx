import React from 'react';
import { ResourceIcon, iconNames } from 'design/ResourceIcons';
import { Flex } from '..';

export default {
  title: 'Design/ResourceIcons',
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
