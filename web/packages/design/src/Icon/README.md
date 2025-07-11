## Adding an Icon to the Library

#### Steps:

1. Download the SVG file of the icon from Google Drive and rename it, ensuring that it is capitalized. This is the name
   that will be used in the app (eg. `ArrowBack.svg`).
    1. PLEASE NOTE: If you are looking at icons in Figma, _please contact the design team to get an
       exported version_. All icons must be converted from vector lines to outlines in order to be
       rendered correctly in the product. Exporting an icon without preparing it will cause
       unpredictable rendering, such as the fill color not being updated or the entire shape being
       filled in with color, rather than just the outlines.
2. Add the file to the `assets/` folder in this directory.
3. Ensure that the icon does not contain unnecessary `clipPath` elements (this will be a single 24x24 rect in a
   `clipPath` element). If it does, delete the `g` element (keeping the `path`) that references the `clipPath`, and the
   `defs` element that contains the `clipPath`. This is necessary to ensure that the icon optimizes correctly (
   see https://github.com/gravitational/teleport/pull/56614 as an example).
4. Run `pnpm process-icons`.

A full collection of exported icons ready for download can be found
in [Google Drive](https://drive.google.com/drive/folders/19068OCcyob6iqjpY3JB4t2NGiRmnuw2D?usp=drive_link). The same
icons and others that require preparation before exporting can be
found [in Figma](https://www.figma.com/file/Gpjs9vjhzUKF1GDbeG9JGE/Application-Design-System?type=design&node-id=7371-35911&mode=design&t=ior9gA5q20atPjr9-0).

If the collection does not include the icon you need, please contact the Design
team or follow the instructions in the "Icon Consistency" section of the above
Figma link to prepare a new icon.
