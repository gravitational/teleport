import React, { useState } from 'react';
import { FieldMultiInput } from './FieldMultiInput';
import Box from 'design/Box';

export default {
  title: 'Shared',
};

export function Story() {
  const [items, setItems] = useState([]);
  return (
    <Box width="500px">
      <FieldMultiInput label="Some items" value={items} onChange={setItems} />
    </Box>
  );
}
Story.storyName = 'FieldMultiInput';
