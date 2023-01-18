import React from 'react';
import { Box, Label } from 'design';

export default function RolesRequested({ roles }: { roles: string[] }) {
  const $roles = roles.sort().map(role => (
    <Label mr="1" key={role} kind="secondary">
      {role}
    </Label>
  ));

  return <Box>{$roles}</Box>;
}
