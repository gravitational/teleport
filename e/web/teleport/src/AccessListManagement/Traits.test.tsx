import {
  convertToTraitConvenience,
  convertTraitLabelsToAllUserTraits,
  deduplicateTraitLabels,
} from './Traits';

test('deduplicateTraitLabels', async () => {
  // Test empty.
  expect(deduplicateTraitLabels([])).toStrictEqual([]);

  const noDuplicates = [
    { name: 'fruit', value: 'apple' },
    { name: 'pet', value: 'llama' },
    { name: 'drink', value: 'coffee' },
  ];
  expect(deduplicateTraitLabels(noDuplicates)).toStrictEqual(noDuplicates);

  const duplicates = [
    ...noDuplicates,
    { name: 'fruit', value: 'banana' },
    { name: 'fruit', value: 'banana' },
    { name: 'drink', value: 'coffee' },
    { name: 'fruit', value: 'banana' },
    { name: 'fruit', value: 'apple' },
    { name: 'drink', value: 'water' },
  ];
  expect(deduplicateTraitLabels(duplicates)).toStrictEqual([
    { name: 'fruit', value: 'apple' },
    { name: 'pet', value: 'llama' },
    { name: 'drink', value: 'coffee' },
    { name: 'fruit', value: 'banana' },
    { name: 'drink', value: 'water' },
  ]);
});

test('convertTraitLabelsToAllUserTraits', async () => {
  // Test empty.
  expect(convertTraitLabelsToAllUserTraits([])).toStrictEqual({});

  const withDuplicates = [
    { name: 'fruit', value: 'apple' },
    { name: 'pet', value: 'llama' },
    { name: 'drink', value: 'coffee' },
    { name: 'fruit', value: 'banana' },
    { name: 'fruit', value: 'banana' },
    { name: 'drink', value: 'coffee' },
    { name: 'fruit', value: 'banana' },
    { name: 'fruit', value: 'apple' },
    { name: 'drink', value: 'water' },
    { name: 'drink', value: 'water' },
    { name: 'fruit', value: 'carrot' },
    { name: 'drink', value: 'water' },
  ];
  expect(convertTraitLabelsToAllUserTraits(withDuplicates)).toStrictEqual({
    fruit: ['apple', 'banana', 'carrot'],
    pet: ['llama'],
    drink: ['coffee', 'water'],
  });
});

test('convertToTraitConvenience', async () => {
  // Test empty.
  expect(convertToTraitConvenience({})).toStrictEqual({
    traitLabels: [],
    traitList: [],
  });

  const data = {
    fruit: ['watermelon', 'dragonfruit', 'apple', 'carrot'],
    pet: ['llama'],
    drink: ['coffee', 'water', 'juice'],
  };
  expect(convertToTraitConvenience(data)).toStrictEqual({
    traitLabels: [
      { name: 'fruit', value: 'watermelon' },
      { name: 'fruit', value: 'dragonfruit' },
      { name: 'fruit', value: 'apple' },
      { name: 'fruit', value: 'carrot' },
      { name: 'pet', value: 'llama' },
      { name: 'drink', value: 'coffee' },
      { name: 'drink', value: 'water' },
      { name: 'drink', value: 'juice' },
    ],
    traitList: [
      'fruit: apple, carrot, dragonfruit, watermelon',
      'pet: llama',
      'drink: coffee, juice, water',
    ],
  });
});
