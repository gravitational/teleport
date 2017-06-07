import React from 'react';

const SizeEnum = {
  'lg': 'grv-line-solid m-t-lg m-b-lg',
  'md': 'grv-line-solid m-t-md m-b-md',
  'sm': 'grv-line-solid m-t-sm m-b-sm',
  'xs': 'grv-line-solid m-t-xs m-b-xs',
}

const Separator = ({size="sm"}) => (
  <div className={ SizeEnum[size] } />
);

export default Separator;