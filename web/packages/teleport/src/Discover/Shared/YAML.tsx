import React from 'react';

import TextEditor from 'shared/components/TextEditor';

export const ReadOnlyYamlEditor = ({ content }: { content: string }) => {
  return <TextEditor readOnly={true} data={[{ content, type: 'yaml' }]} />;
};
