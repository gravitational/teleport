import React from 'react';
import Box from 'app/components/common/boxes/box';

export const EmptyList = props => (
  <Box> 
    <div className="text-center" style={{ minHeight: "50px", margin: "25px auto", maxWidth: "600px" }}>
      <p>
        <strong>You do not have anything here</strong>
      </p>
      <div className="text-muted"> 
        <div>          
            <button className="btn btn-sm btn-primary" onClick={props.onClick}>          
              Create
            </button>
          </div>            
        </div>        
    </div>    
  </Box>  
);
