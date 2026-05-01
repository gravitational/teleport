import {
  AccessListDescriptor,
  AccessListType,
} from 'e-teleport/services/accessmanagement';
import { AccessListPreset } from 'e-teleport/services/accessmanagement/preset';

import { isGuideEditorSupported } from './useGuideEditor';

function makeDescriptor(
  type: AccessListType,
  preset: AccessListPreset,
  labels: Record<string, string> = {}
): AccessListDescriptor {
  return {
    type,
    preset,
    metadata: { name: 'test', revision: '', labels },
  };
}

const terraformLabel = { 'teleport.dev/iac-tool': 'terraform' };

describe('isGuideEditorSupported', () => {
  test('static access list with terraform label', () => {
    expect(
      isGuideEditorSupported(
        makeDescriptor(AccessListType.Static, 'long-term', terraformLabel)
      )
    ).toBe(true);
    expect(
      isGuideEditorSupported(
        makeDescriptor(AccessListType.Static, 'short-term', terraformLabel)
      )
    ).toBe(true);
  });

  test('static access list with terraform label but no preset returns', () => {
    expect(
      isGuideEditorSupported(
        makeDescriptor(AccessListType.Static, '', terraformLabel)
      )
    ).toBe(false);
  });

  test('static access list without terraform label with supported preset', () => {
    expect(
      isGuideEditorSupported(makeDescriptor(AccessListType.Static, 'long-term'))
    ).toBe(false);
  });

  test('default access list with supported preset', () => {
    expect(
      isGuideEditorSupported(
        makeDescriptor(AccessListType.Default, 'long-term')
      )
    ).toBe(true);

    expect(
      isGuideEditorSupported(
        makeDescriptor(AccessListType.Default, 'short-term')
      )
    ).toBe(true);
  });

  test('default access list with no preset', () => {
    expect(
      isGuideEditorSupported(makeDescriptor(AccessListType.Default, ''))
    ).toBe(false);
  });

  test('unsupported access list type regardless of preset', () => {
    expect(
      isGuideEditorSupported(makeDescriptor(AccessListType.Scim, 'long-term'))
    ).toBe(false);
  });
});
