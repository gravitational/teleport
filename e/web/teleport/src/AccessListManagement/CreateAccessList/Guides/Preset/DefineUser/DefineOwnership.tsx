import { MaxWidthBox } from 'e-teleport/AccessListManagement/GuideEditor/Shared';

import { DefineUserTemplate } from './DefineUserTemplate';

export function DefineOwnership() {
  return (
    <MaxWidthBox>
      <DefineUserTemplate userKind="owner" isLastStep={true} />
    </MaxWidthBox>
  );
}
