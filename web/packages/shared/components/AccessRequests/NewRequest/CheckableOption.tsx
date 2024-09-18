import React from 'react';
import { Flex, Text } from 'design';
import { components, OptionProps } from 'react-select';

import { Option } from 'shared/components/AccessRequests/NewRequest/resource';

export const CheckableOption = (
  props: OptionProps<Option> & { data: Option }
) => {
  const { data } = props;
  return (
    <components.Option {...props}>
      <Flex alignItems="center" py="8px" px="12px">
        <input
          type="checkbox"
          checked={props.isSelected}
          readOnly
          name={data.value}
          id={data.value}
        />{' '}
        <Text ml={1}>{data.label}</Text>
      </Flex>
    </components.Option>
  );
};
