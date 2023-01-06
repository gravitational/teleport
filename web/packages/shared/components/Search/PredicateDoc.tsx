import React from 'react';
import { Text, Link } from 'design';

const GUIDE_URL =
  'https://goteleport.com/docs/setup/reference/predicate-language/#resource-filtering';

export const PredicateDoc = () => (
  <>
    <Text typography="paragraph2">
      Advanced search allows you to perform more sophisticated searches using
      the predicate language. The language supports the basic operators:{' '}
      <Text as="span" bold>
        <code>==</code>{' '}
      </Text>
      ,{' '}
      <Text as="span" bold>
        <code>!=</code>
      </Text>
      ,{' '}
      <Text as="span" bold>
        <code>&&</code>
      </Text>
      , and{' '}
      <Text as="span" bold>
        <code>||</code>
      </Text>
    </Text>
    <Text typography="h4" mt={2} mb={1}>
      Usage Examples
    </Text>
    <Text typography="paragraph2">
      Label Matching:{' '}
      <Text ml={1} as="span" bold>
        <code>labels["key"] == "value" && labels["key2"] != "value2"</code>{' '}
      </Text>
      <br />
      Fuzzy Searching:{' '}
      <Text ml={1} as="span" bold>
        <code>search("foo", "bar", "some phrase")</code>
      </Text>
      <br />
      Combination:{' '}
      <Text ml={1} as="span" bold>
        <code>labels["key1"] == "value1" && search("foo")</code>
      </Text>
    </Text>
    <Text typography="paragraph2" mt={2}>
      Check out our{' '}
      <Link href={GUIDE_URL} target="_blank">
        predicate language guide
      </Link>{' '}
      for a more in-depth explanation of the language.
    </Text>
  </>
);
