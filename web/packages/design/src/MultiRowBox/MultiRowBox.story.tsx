import React from 'react';

import { MultiRowBox, Row, SingleRowBox } from './MultiRowBox';

export default {
  title: 'Design/MultiRowBox',
  component: MultiRowBox,
};

export const WithMultipleRows = () => (
  <MultiRowBox>
    <Row>Part 1</Row>
    <Row>Part 2</Row>
    <Row>Part 3</Row>
  </MultiRowBox>
);

export const WithSingleRow = () => (
  <SingleRowBox>
    <div>Hello,</div>
    <div>World!</div>
  </SingleRowBox>
);
