# Gravitational Design System

This package contains the source code of Gravitational Design System

## Intro

As CSS-in-JS libraries matured and with the emergence of such great tools as
[storybook](https://storybook.js.org/), it became significantly easier and faster to
build UI components and design systems.

We looked at other design-systems and we realized that it's easier to
build it ourselves rather than trying to customize/override the third-party
components.

This design system has a limited feature set designed around Gravitational requirements.

This design system takes advantage of the two great libraries
[styled-components](https://github.com/styled-components/styled-components)
and [styled-system](https://github.com/styled-system/styled-system).

Please refer to the following articles to get familiar with the general
concept and principles on which this design system is built.

[Build Better Component Libraries with Styled System](https://medium.com/styled-components/build-better-component-libraries-with-styled-system-4951653d54ee)

[Component Based Design System With Styled-System](https://varun.ca/styled-system/)

## Typography

Here are some general rules for using typography in our apps:

### General order of preference

1. Prefer using semantic wrappers like `<H1/>`, `<P/>` to `<Text
   typography="..."/>` if we can assign semantics to a given piece of content.
2. Otherwise, prefer `<Text typography="..."/>` to using custom font sizes and
   other text attributes.
3. Only use custom font sizes and other text attributes when necessary.

### Rules for Specific Components

#### Headings and Subtitles

- Headings: `<H1/>`, `<H2/>`, `<H3/>`, `<H4/>` wrappers render actual HTML tags,
  which is better for accessibility. Usage examples:
  - `<H1/>` — page titles, empty result set notifications followed up by
    additional content.
  - `<H2/>` — dialog titles, dialog-like side panel titles, resource enrollment
    subpage titles.
  - `<H3/>` — explanatory side panel titles, resource enrollment step boxes.
- Don’t use semantic heading wrappers if the heading typography is used only to
  make text bigger or bolder or otherwise stand out, and it doesn’t introduce
  structure to the page with even a brief follow-up content. In this case,
  either use an existing body typography and potentially additional `bold`
  attribute, or revert to using `<Text typography="h?"/>`.
- To create subtitles for `<H1/>` through `<H3/>` headings, use `<Subtitle1/>`
  through `<Subtitle3/>` components. Don’t use subtitle typography for more than
  a short subtitle right next to the heading.
- When creating a heading with a subtitle, it’s a good practice to wrap them
  both in a `<header>` HTML element.
- In some justified cases, it’s valid to use a semantic heading and override
  typography settings on it or its part. This way, we retain the accessible
  document structure *and* achieve the visual effect that we need. Just make
  sure that whatever text formatting you use, it’s easily understandable as a
  heading.

#### Paragraphs

- Use `<P/>` if your text consists of actual sentences. This component
  automatically handle inter-paragraph spacing without disrupting the external
  margins. It doesn’t enforce any typography, so it defaults to whatever is
  inherited from the component (`body2` is the global default).
- Do *not* use `<br/>` unless a line break is exactly what is appropriate (e.g.
  in code or command snippets). Prefer paragraphs or other block elements to
  separate text vertically.
- Use `<P1/>` through `<P3/>` for paragraphs that explicitly override typography
  to `body1` through `body3`, respectively. Note that picking between them
  doesn’t have anything to do with their level in document’s heading hierarchy;
  having multiple versions of these is only a shortcut for applying different
  body typography to paragraphs.

### Other Advice

- Use proper title capitalization on headings and buttons, except where text is
  clearly meant to be a full sentence. Do it even if you’re using `caps`
  modifier or the `<H4/>` component, which uses all caps for its content; this
  way, it will be easier to refactor your code.
- This document was created a long time after we started working on our UI, and
  after a redesign. You may see a lot of legacy code that doesn’t follow these
  rules. If you work with such, consider spending a couple of minutes to fix it!
  Consistency is important.