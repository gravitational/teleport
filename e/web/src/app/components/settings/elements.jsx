import React from 'react';
import Box from '../common/boxes/box';
import Button from '../common/button';

export const NewButton = props => (
  <Button
    size="sm"
    isDisabled={!props.enabled}
    onClick={props.onClick}
    className="grv-settings-res-new m-t btn-default">
    <i className="fa fa-plus m-r-xs"/>{props.text}
  </Button>
)

export const EmptyList = ({canCreate=true, onClick}) => (
  <EmptyBox>
    <div className="text-center">
      <p>
        You do not have anything here
      </p>
      <NewButton enabled={canCreate}text="Create" onClick={onClick}/>
    </div>
  </EmptyBox>
);

export const EmptyBox = props => (
  <Box className="grv-settings-empty">
    {props.children}
  </Box>
)