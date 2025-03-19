import { defineSlotRecipe } from "@chakra-ui/react"

export const collapsibleSlotRecipe = defineSlotRecipe({
  slots: ["root", "trigger", "content"],
  className: "chakra-collapsible",
  base: {
    content: {
      overflow: "hidden",
      _open: {
        animationName: "expand-height, fade-in",
        animationDuration: "moderate",
      },
      _closed: {
        animationName: "collapse-height, fade-out",
        animationDuration: "moderate",
      },
    },
  },
})
