import React from 'react';
import Button from 'app/components/common/button';

const Empty = ({ onClick, title, description, btnText }) => (
  <div className="text-center" style={{ minHeight: "50px", margin: "25px auto", maxWidth: "600px" }}>
    <p>
      {title}
    </p>
    <div className="text-muted"> {description}</div>
    { btnText && <Button
      className="btn-sm btn-primary m-t"
      onClick={onClick}>
      {btnText}
    </Button>
    }
  </div>
);

export default Empty;