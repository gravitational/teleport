/*
Copyright 2019 Gravitational, Inc.

Licensed under the Apache License, Version 2.0 (the "License");
you may not use this file except in compliance with the License.
You may obtain a copy of the License at

    http://www.apache.org/licenses/LICENSE-2.0

Unless required by applicable law or agreed to in writing, software
distributed under the License is distributed on an "AS IS" BASIS,
WITHOUT WARRANTIES OR CONDITIONS OF ANY KIND, either express or implied.
See the License for the specific language governing permissions and
limitations under the License.
*/

const light = 300;
const regular = 400;
const bold = 600;

export const fontSizes = [10, 12, 14, 16, 18, 20, 22, 24, 26, 28, 34];

export const fontWeights = { light, regular, bold };

const typography = {
  h1:{
    fontWeight: light,
    fontSize: "34px",
    lineHeight: '56px',
  },
  h2: {
    fontWeight: light,
    fontSize: "26px",
    lineHeight: '40px',
  },
  h3: {
    fontWeight: regular,
    fontSize: "20px",
    lineHeight: '32px',
  },
  h4:{
    fontWeight: regular,
    fontSize: "18px",
    lineHeight: '32px',
  },
  h5:{
    fontWeight: regular,
    fontSize: "16px",
    lineHeight: '24px',
  },
  h6:{
    fontWeight: bold,
    fontSize: "14px",
    lineHeight: '24px',
  },
  body1:{
    fontWeight: regular,
    fontSize: "14px",
    lineHeight: '24px',
  },
  body2:{
    fontWeight: regular,
    fontSize: "12px",
    lineHeight: '16px',
  },
  paragraph:{
    fontWeight: light,
    fontSize: "16px",
    lineHeight: '32px',
  },
  paragraph2:{
    fontWeight: light,
    fontSize: "12px",
    lineHeight: '24px',
  },
  subtitle1:{
    fontWeight: regular,
    fontSize: "14px",
    lineHeight: '24px',
  },
  subtitle2:{
    fontWeight: bold,
    fontSize: "10px",
    lineHeight: '16px',
  }
}

export default typography;