# ResourceIcon

Brand / resource icons (databases, apps, integrations, …), rendered via `<ResourceIcon name="…" />`.

Theming is driven **entirely by CSS** through the `html.dark` / `html.light` class set by `ConfiguredThemeProvider`.

## The three kinds of icon

The build step (`pnpm process-icons`) classifies each SVG in `assets/` automatically:

| Kind       | When                                                                                                                                        | How it renders                                                                                                                     |
|------------|---------------------------------------------------------------------------------------------------------------------------------------------|------------------------------------------------------------------------------------------------------------------------------------|
| **mono**   | a `name-dark.svg` / `name-light.svg` pair that differs only by a single flipping color, **or** a single `name.svg` that uses `currentColor` | one `currentColor` SVG compiled to a React component in `MonoIcons/`, colored via CSS (defaults to black in light / white in dark) |
| **themed** | a `name-dark.svg` / `name-light.svg` pair with genuinely different artwork (multi-color, gradients, distinct shapes)                        | a single `<img>` whose asset is swapped via CSS `content` on `html.light`                                                          |
| **static** | a single full-color `name.svg`                                                                                                              | a plain `<img>`                                                                                                                    |

## Adding or changing an icon

1. Drop the SVG(s) into `assets/`:
   - a monochrome icon: a single `currentColor` `foo.svg`, **or** a `foo-dark.svg` + `foo-light.svg` pair (the build 
     auto-collapses pure recolors into one `currentColor` SVG),
   - a two-artwork icon: a `foo-dark.svg` + `foo-light.svg` pair,
   - a full-color logo: a single `foo.svg`.
2. Run `pnpm process-icons`. This optimizes the SVGs (SVGO), collapses monochrome pairs, and regenerates the
   `MonoIcons/` components, `icons.ts` (URL exports for non-mono icons), and `icons.json` (brand colors). It warns if a
   generated mono icon has no `resourceIconSpecs.ts` entry yet.
3. Add one entry to `resourceIconSpecs.ts` mapping the icon name to its spec (import names in `icons.ts` are the
   camel-cased filename, e.g. `foo-dark.svg` → `i.fooDark`):
   - `foo: mono(Icons.Foo)`
   - `foo: themed(i.fooDark, i.fooLight)`
   - `foo: staticIcon(i.foo)`
   - aliases just reuse another icon, e.g. `chatgpt: mono(Icons.Openai)`.

You never need to edit `icons.ts` - it's regenerated from `assets/` on every run.

`resourceIconSpecs.ts` is the single hand-authored registry: it holds the curated icon names (which are matched by 
`guessAppIcon`, so keep them close to real product names) and any aliases. 

Everything else - the `MonoIcons/` components, `icons.ts` URL exports, and `icons.json` colors - is generated.

## Brand colors

A monochrome icon defaults to black (light theme) / white (dark theme). If a brand wants a specific color, it is read 
from the original pair and stored in `icons.json`, e.g.:

```json
{
  "colors": {
    "linkedin": { 
      "dark": "#fff",
      "light": "#0a66c2"
    }
  }
}
```

The generated component then bakes those colors in (applied per theme via the `<html>` class). `icons.json` is 
generated/updated by `pnpm process-icons`, but you can also edit it by hand to tweak a color.

## Generated vs hand-authored

Generated (do not edit): `MonoIcons/` and `icons.ts`.

Hand-authored: `resourceIconSpecs.ts`, the SVGs in `assets/`, and the base components `MonoIcon.tsx` / `ThemedImage.tsx`.

`icons.json` is generated (brand colors are extracted from the dark/light pair when an icon is collapsed), but you may
hand-tweak a brand color in it - values are preserved across runs.
