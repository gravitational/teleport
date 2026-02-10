/**
 * Teleport
 * Copyright (C) 2025 Gravitational, Inc.
 *
 * This program is free software: you can redistribute it and/or modify
 * it under the terms of the GNU Affero General Public License as published by
 * the Free Software Foundation, either version 3 of the License, or
 * (at your option) any later version.
 *
 * This program is distributed in the hope that it will be useful,
 * but WITHOUT ANY WARRANTY; without even the implied warranty of
 * MERCHANTABILITY or FITNESS FOR A PARTICULAR PURPOSE.  See the
 * GNU Affero General Public License for more details.
 *
 * You should have received a copy of the GNU Affero General Public License
 * along with this program.  If not, see <http://www.gnu.org/licenses/>.
 */

import { Meta, StoryObj } from '@storybook/react-vite';

import { Markdown } from './Markdown';

const meta = {
  title: 'Shared/Markdown',
  component: Wrapper,
} satisfies Meta<typeof Wrapper>;

type Story = StoryObj<typeof meta>;

export default meta;

const sampleDoc = `# Markdown Syntax Demo

This document demonstrates basic markdown syntax elements.

## Headers

# Header 1
## Header 2
### Header 3
#### Header 4
##### Header 5
###### Header 6

## Text Formatting

**Bold text** or __bold text__

*Italic text* or _italic text_

***Bold and italic*** or ___bold and italic___

~~Strikethrough text~~

\`Inline code\`

## Lists

### Unordered Lists
- Item 1
- Item 2
  - Nested item 2.1
  - Nested item 2.2
* Alternative bullet
+ Another alternative

### Ordered Lists
1. First item
2. Second item
   1. Nested item 2.1
   2. Nested item 2.2
3. Third item

## Links and Images

[Link text](https://example.com)

[Link with title](https://example.com "This is a title")

![Alt text for image](https://via.placeholder.com/150)

![Image with title](https://via.placeholder.com/150 "Image title")

## Code Blocks

\`\`\`
Plain code block
with multiple lines
\`\`\`

\`\`\`javascript
// JavaScript code block
function hello() {
    console.log("Hello, world!");
}
\`\`\`

## Blockquotes

> This is a blockquote.
>
> It can span multiple paragraphs.

> Nested blockquotes
>> Like this one

## Tables

| Column 1 | Column 2 | Column 3 |
|----------|----------|----------|
| Row 1    | Data     | More     |
| Row 2    | Data     | More     |

| Left | Center | Right |
|:-----|:------:|------:|
| Left aligned | Centered | Right aligned |

## Horizontal Rules

---

***

___

## Line Breaks and Paragraphs

This is a paragraph.

This is another paragraph separated by a blank line.

This line ends with two spaces
This creates a line break without a paragraph break.

## Escape Characters

\\*This won't be italic\\*

\\# This won't be a header

Use backslashes to escape special characters: \\\\ \\\` \\* \\_ \\{ \\} \\[ \\] \\( \\) \\# \\+ \\- \\. \\!`;

export const Basic: Story = {
  args: {
    markdown: sampleDoc,
    enableLinks: true,
  },
};

export const WithoutLinks: Story = {
  args: {
    markdown: sampleDoc,
    // enableLinks: false, // links are hidden by default.
  },
};

function Wrapper(props: { markdown: string; enableLinks?: boolean }) {
  return <Markdown text={props.markdown} enableLinks={props.enableLinks} />;
}
