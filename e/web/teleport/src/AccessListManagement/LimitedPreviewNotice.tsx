import React from 'react';
import { Info } from 'design/Alert';
import cfg from 'teleport/config';

export function LimitedPreviewNotice() {
  if (cfg.isCloud) {
    return null;
  }

  return (
    <Info>
      We're offering this feature in preview and will limit it in future
      versions.
    </Info>
  );
}
