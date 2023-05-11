import { Appearance } from '@stripe/stripe-js';

/**
 * Returns the configured Stripe appearance object
 *
 * @remarks
 * Changes to this object will impact all Stripe elements (payment, address, etc.)
 *
 * @param theme - provided by the parent via the useTheme hook
 */
export const getAppearance = (theme): Appearance => ({
  variables: {
    fontFamily: theme.fontFamily,
    colorBackground: theme.colors.levels.surface,
    colorText: theme.colors.text.main,
    colorDanger: theme.colors.error.main,
    colorPrimary: theme.colors.brand,
    spacingUnit: '4px',
    borderRadius: '8px',
  },
});
