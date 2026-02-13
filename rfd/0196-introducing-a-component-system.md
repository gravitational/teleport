---
authors: Ryan Clark (ryan.clark@goteleport.com)
state: draft
---

# RFD 0194 - Introducing a component system

## Required Approvers

* Engineering: @ravicious || @kimlisa || @avatus
* Design: @roraback

## What

Introduce Chakra UI, a 3rd-party component system with pre-built components and easy styling capabilities into our web
UI.

## Why

Modernise our ability to create styled, reusable, useful components without having to reinvent them from scratch each
time. Speed up development time by having pre-built, tested components ready to use with minimal fuss. Accessibility,
including ARIA support and keyboard navigation, is built in to all components.

## Current Setup

Our current design system is using `styled-system` and `styled-components` to create both reusable components and all the
views/pages in the web UI. These are both great libraries, but there are a few drawbacks to the current usage which
seems them
being either underused (`styled-system`), or overused (`styled-components`).

### styled-system vs styled-components

Components defined via `styled-system` are essentially "building blocks" and can accept a lot of different props (some
with shorthand values)
to define their styles.

```tsx
<Box
  px={2}
  py={1}
  bg="levels.popout"
  border="1px solid"
  borderColor="alpha.200"
  borderRadius="md"
>
  Some content!
</Box>
```

Components defined via `styled-components` are more like "display components", typically taking only a few (if not no)
props and having its style defined
in a template literal.

```tsx
const StyledBox = styled.div`
  padding: ${p => p.theme.space[2]}px ${p => p.theme.space[1]}px;
  background: ${p => p.theme.colors.levels.popout};
  border: 1px solid ${p => p.theme.colors.spotBackground[2]};
  border-radius: ${p => p.theme.radii[2]}px;
`;
```

## Drawbacks of current setup

### A lot of it is untyped

A lot of the components are untyped, which results in no autocomplete for props when using them in a component.

When using the `Link` component from the design system, the only properties offered to you in the autocomplete is `key`
and `css`.

_Note: this isn't a major issue, as we have added types to a lot of the components, but adding them to the remaining components
is a lot of cruft work that impedes developer velocity._

### Manual definition

All the building blocks - `Box`, `Flex`, etc, are manually defined, like so:

```tsx
export interface BoxProps
  extends MaxWidthProps,
    MinWidthProps,
    SpaceProps,
    HeightProps,
    LineHeightProps,
    MinHeightProps,
    MaxHeightProps,
    WidthProps,
    ColorProps,
    TextAlignProps,
    FlexProps,
    AlignSelfProps,
    JustifySelfProps,
    BorderProps,
    BordersProps,
    OverflowProps {
}

const Box = styled.div<BoxProps>`
  box-sizing: border-box;
  ${maxWidth}
  ${minWidth}
  ${space}
  ${height}
  ${lineHeight}
  ${minHeight}
  ${maxHeight}
  ${width}
  ${color}
  ${textAlign}
  ${flex}
  ${alignSelf}
  ${justifySelf}
  ${borders}
  ${overflow}
`;
```

This results in a lot of unnecessary boilerplate for components, as well as components not necessarily implementing all
the props that you may want to use - the above example lacking the ability for any positioning related props (`left`,
`right`, `position`, etc.) and many other possible props.

### styled-system is underused, styled-components is slow (to write)

Instead of using `styled-system`, most of our display components are using `styled-components` to define their styles.

It's not clear why the codebase has evolved to use this, but it is a much slower way of writing styles, both for the
developer and the browser.

Currently, to define an element that has padding, a background color, a border and border radius:

```tsx
const StyledBox = styled.div`
  padding: ${p => p.theme.space[2]}px ${p => p.theme.space[1]}px;
  background: ${p => p.theme.colors.levels.popout};
  border: 1px solid ${props => props.theme.colors.spotBackground[2]};
  border-radius: ${props => props.theme.radii[2]}px;
`;

function Component() {
  return (
    <StyledBox>
      Some content!
    </StyledBox>
  );
}
```

Using `styled-system` more could be a good solution here, but we still have a few more issues that can be solved by
bringing in Chakra UI.

```tsx
function Component() {
  return (
    <Box
      px={2}
      py={1}
      bg="levels.popout"
      border="1px solid"
      borderColor="alpha.200"
      borderRadius="md"
    >
      Some content!
    </Box>
  );
}

// in a reusable way
function StyledBox({ children, ...rest }: PropsWithChildren<BoxProps>) {
  return (
    <Box
      px={2}
      py={1}
      bg="levels.popout"
      border="1px solid"
      borderColor="alpha.200"
      borderRadius="md"
      {...rest}
    >
      {children}
    </Box>
  );
}
```

### Theming components isn't ideal

We style our components through (complicated) functions that return partial parts of the style from the given props.

For example, our `Button` component - which deals with multiple variants (`fill`, `intent`), uses a function to generate
different objects depending on the given variants.

```ts
const buttonPalette = <E extends React.ElementType>({
  theme: { colors },
  intent,
  fill,
}: ThemedButtonProps<E>): ButtonPalette => {
  switch (fill) {
    case 'filled':
      if (intent === 'neutral') {
        return {
          default: {
            text: colors.text.slightlyMuted,
            background: colors.interactive.tonal.neutral[0],
          },
          hover: {
            text: colors.text.main,
            background: colors.interactive.tonal.neutral[1],
          },
          active: {
            text: colors.text.main,
            background: colors.interactive.tonal.neutral[2],
          },
          focus: {
            text: colors.text.slightlyMuted,
            border: colors.text.slightlyMuted,
            background: colors.interactive.tonal.neutral[0],
          },
        };
      } else {
        return {
          default: {
            text: colors.text.primaryInverse,
            background: colors.interactive.solid[intent].default,
          },
          hover: {
            text: colors.text.primaryInverse,
            background: colors.interactive.solid[intent].hover,
          },
          active: {
            text: colors.text.primaryInverse,
            background: colors.interactive.solid[intent].active,
          },
          focus: {
            text: colors.text.primaryInverse,
            border: colors.text.primaryInverse,
            background: colors.interactive.solid[intent].default,
          },
        };
      }
    case 'minimal': {
      if (intent === 'neutral') {
        return {
          default: {
            text: colors.text.slightlyMuted,
            background: 'transparent',
          },
          hover: {
            text: colors.text.slightlyMuted,
            background: colors.interactive.tonal.neutral[0],
          },
          active: {
            text: colors.text.main,
            background: colors.interactive.tonal.neutral[1],
          },
          focus: {
            text: colors.text.slightlyMuted,
            border: colors.text.slightlyMuted,
            background: 'transparent',
          },
        };
      }
      return {
        default: {
          text: colors.interactive.solid[intent].default,
          background: 'transparent',
        },
        hover: {
          text: colors.interactive.solid[intent].hover,
          background: colors.interactive.tonal[intent][0],
        },
        active: {
          text: colors.interactive.solid[intent].active,
          background: colors.interactive.tonal[intent][1],
        },
        focus: {
          text: colors.interactive.solid[intent].default,
          border: colors.interactive.solid[intent].default,
          background: 'transparent',
        },
      };
    }
    // border...
  }
};
```

This is just for dealing with the text, border and background. There's also `buttonStyle` which defines some defaults,
`themedStyles` which pulls in some features
from `styled-system` to allow a subset of props to be passed in, and `size` which defines the size of the button.

Using recipes from Chakra UI would allow us to define this somewhat like this:

```tsx
export const styledButton = defineRecipe({
  base: {
    border: '1px solid',
    borderColor: 'alpha.200',
    borderRadius: 'md',
  },
  variants: {
    fill: {
      filled: {
        color: 'text.primaryInverse',
      },
      // etc...
    },
    intent: {
      primary: {
        color: 'interactive.solid.primary.default',
      },
      // etc...
    },
    size: {
      sm: { px: 2, py: 1, fontSize: '12px' },
      lg: { px: 3, py: 2, fontSize: '24px' },
    },
  },
})
// I tried to implement a full example of the button but couldn't follow the current definition
```

### Our building blocks aren't customisable enough

Similar to above, due to us not using `styled-system` enough, it's common to see patterns like:

```tsx
const PreviewWrapper = styled(Box)`
  border-radius: ${p => p.theme.radii[3]}px;
  box-shadow: ${p => p.theme.boxShadow[1]};
  transform: var(--feature-preview-scale);
  background-color: ${p => p.theme.colors.levels.surface};
`;
```

Due to the `Box` component not accepting all props that developers expect. This could just be:

```tsx
<Box
  borderRadius="lg"
  boxShadow="md"
  transform="var(--feature-preview-scale)"
  backgroundColor="levels.surface"
>
  ...
</Box>
```

The first code example takes longer to write, as it has to be defined outside the component and then referred to
in the component. A name has to be thought up, and to use values from the theme, a longer syntax of `${p => p.theme.whatever.is.needed[2]}` 
has to be used, instead of simply using `value="whatever.is.needed.2"`. This also takes longer for a developer coming back to the code
to understand where the styling is.

### Our components are custom-made

We currently have to reinvent the wheel a lot of the time when creating components. This is fine for some components, but
we're missing accessibility and features that are built into pre-built components. Our tooltip component lacks the ability
to know if the tooltip will be displayed off the screen, and our modal component doesn't have the ability to close with the 
`Esc` key. 

Using pre-built components from Chakra UI would enhance developer velocity, as they do not have to worry
about implementing these features.

## Advantages of using Chakra UI

Chakra UI is a React component system that has been around for over 5 years, with 38k stars on GitHub and a large
community around it. There are Chakra versions of other 3rd-party libraries we use, such as `react-select`, that allow
for consistent theming and styling across the codebase.

Chakra is a framework that is very similar to how we currently write components, making it an easier fit into the codebase
without having to relearn a new way of writing components. 

### Pre-built components

Chakra UI has [a lot of pre-built components](https://chakra-ui.com/docs/components/concepts/overview) that are ready to
use, and offer better implementations than our current custom-made components (Tooltip, Modal, etc.).

As we introduce Chakra UI components into the Teleport codebase, the recipes will need to be altered to match the
Teleport designs. This should be pretty straightforward, and most of the time be spacing/color changes.

### Easy styling

As mentioned above, Chakra adds the ability to create recipes, which allow the definition of reusable components to
stay in
one place, and be reused throughout the codebase. Recipes also make defining dark mode variants much easier.

```tsx
export const styledButton = defineRecipe({
  base: {
    border: '1px solid',
    borderColor: {
      _dark: 'alpha.100',
      value: 'alpha.200',
    },
    borderRadius: 'md',
  },
})
```

### CSS variables everywhere

Chakra UI exports all defined colors, fonts, spacing, radii, etc. as CSS variables, which means that they can be
referred to
when not using Chakra. CSS variables are faster for the browser to parse over grabbing values from the theme prop in a styled component.

If a developer still wishes to use `styled.x`, they can refer to the CSS variables directly, like so:

```tsx
const StyledBox = styled.div`
  padding: var(--teleport-space-1) var(--teleport-space-1);
  background: var(--teleport-colors-levels-popout);
  border: 1px solid var(--teleport-colors-alpha-200);
  border-radius: var(--teleport-radii-2);
`;
```

However, this is not recommended, as there is no validation of the CSS variables.

### Supports code generation for types

All variants, sizes, etc. are generated as types, which means that the developer experience is much better when using
components defined from Chakra UI, or built on top of it. Theming props are auto generated for custom recipes, so the
different variant props can be type-checked and autocompleted during development.

## Reasonable approach to adoption

### First step

After a small proof of concept, full stack resources would be needed to style some of the existing Chakra components to
Teleport's theme. This can also be a great opportunity for everyone to try it out and get to grips with Chakra.

### Code structure and approach

[RFD 206](https://github.com/gravitational/teleport/pull/53359) explains how we can approach the code structure changes needed to implement this. 

### Introduce new pages with the new design system

In an ideal world, putting together new pages should be easy - taking a few reusable, common components and wiring them up.

Working with the full stack team, we can gather what they components/design items they commonly reuse, and what they create each time, and start
building out the new design system with these components in mind.

## Alternatives

### Ark UI

Ark UI is a library that is also by the creators of Chakra UI, which provides a library of un-styled components and their functionality.

As Chakra UI is built on top of this, it would be a lot of work to implement the same styling capabilities that Chakra UI
offers, just to get to the same point.

### MUI

MUI is a very popular component system, with a lot of components and a large community. However, it is a lot more opinionated
than Chakra UI - all components follow the complete Material UI design system, which would require more work to
get them to match the Teleport design system.

## Chakra usage in Teleport Identity Security

Chakra UI is the component system used in Teleport Identity Security, and has been a great productivity booster
due to the pre-built components and easy styling capabilities. Using Chakra in the Teleport codebase would provide
a consistent design system that can be used by all Teleport products.
