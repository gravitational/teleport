import React from 'react';
import { ButtonPrimary, ButtonBorder } from 'design';

export const RequestButton = ({
  isAgentAdded,
  toggleAgent,
}: {
  isAgentAdded: boolean;
  toggleAgent: () => void;
}) => {
  if (isAgentAdded) {
    return (
      <ButtonPrimary width="134px" size="small" onClick={toggleAgent}>
        Remove
      </ButtonPrimary>
    );
  }
  return (
    <ButtonBorder width="134px" size="small" onClick={toggleAgent}>
      + Add to request
    </ButtonBorder>
  );
};
