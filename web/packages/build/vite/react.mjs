import react from '@vitejs/plugin-react-swc';

/** @param {boolean} development */
export const commonConfig = development => {
  return react({
    plugins: [['@swc/plugin-styled-components', getStyledComponentsConfig(development)]],
  });
};

/** @param {boolean} development */
function getStyledComponentsConfig(development) {
  // https://nextjs.org/docs/advanced-features/compiler#styled-components
  if (!development) {
    return {
      ssr: false,
      pure: false, // not currently supported by SWC
      displayName: false,
      fileName: false,
      cssProp: true,
    };
  }

  return {
    ssr: false,
    pure: true, // not currently supported by SWC
    displayName: true,
    fileName: true,
    cssProp: true,
  };
}
