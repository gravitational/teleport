import { AccessListType, isReviewable } from './types';

describe('isReviewable', () => {
  test('should return true for AccessListType.(Implicit)Dynamic', () => {
    expect(isReviewable(AccessListType.Default)).toBe(true);
    expect(isReviewable(AccessListType.Static)).toBe(false);
    expect(isReviewable(AccessListType.Scim)).toBe(true);
  });
});
