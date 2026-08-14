import { Mark, Text } from 'design';

import { AccessListPreset } from 'e-teleport/services/accessmanagement/preset';

export function PresetDescription({ preset }: { preset: AccessListPreset }) {
  return (
    <Text mb={3}>
      Defines what resources <Mark>members</Mark> have access to.
      <br />
      {preset === 'long-term' && (
        <>
          Members are granted <Mark>standing access</Mark> to resources granted
          by this access list
        </>
      )}
      {preset === 'short-term' && (
        <>
          Members need to request for <Mark>temporary access</Mark> to resources
          granted by this access list.
        </>
      )}
    </Text>
  );
}
