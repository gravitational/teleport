const presets = ['@babel/preset-env', '@babel/preset-react'];

const plugins = [
  '@babel/plugin-proposal-class-properties',
  '@babel/plugin-proposal-object-rest-spread',
  '@babel/plugin-syntax-dynamic-import',
];

module.exports = {
  env: {
    test: {
      presets: [
        ['@babel/preset-env', { targets: { node: 'current' } }],
        '@babel/preset-react',
      ],
    },
    development: {
      plugins: [
        'react-hot-loader/babel',
        ...plugins,
        'babel-plugin-styled-components',
      ],
    },
  },
  presets,
  plugins,
};
