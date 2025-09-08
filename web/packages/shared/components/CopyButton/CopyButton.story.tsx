import React from 'react';

import Input from 'design/Input';

import { CopyButton as CopyButtonComponent } from './CopyButton';

export default {
  title: 'Shared',
  component: CopyButtonComponent,
};

export const CopyButton = () => (
  <div>
    <CopyButtonComponent value="Surprise!" />
    Paste here to test: <Input />
  </div>
);

CopyButton.storyName = 'CopyButton';
