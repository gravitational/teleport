import React from 'react';
import Box from 'app/components/common/boxes/box';
import Button from 'app/components/common/button';

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
  <Box> 
    <div className="text-center" style={{ minHeight: "50px", margin: "25px auto", maxWidth: "600px" }}>
      <p>
        <strong>You do not have anything here</strong>
      </p>      
      <NewButton enabled={canCreate}text="Create" onClick={onClick}/>                  
    </div>    
  </Box>  
);
