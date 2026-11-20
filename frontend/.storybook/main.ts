import type { StorybookConfig } from '@storybook/angular-vite';

const config: StorybookConfig = {
  "stories": [
    "../src/**/*.mdx",
    "../src/**/*.stories.@(js|jsx|mjs|ts|tsx)"
  ],
  "addons": [    {
      name: "addon-storybook-sync",
      options: {
        port: Number(process.env.WEBSTORM_ADDON_STORYBOOK_SYNC_PORT) || undefined,
      },
    },
    "@storybook/addon-vitest",
    "@storybook/addon-a11y",
    "@storybook/addon-docs",
  ],
  "framework": "@storybook/angular-vite"
};
export default config;