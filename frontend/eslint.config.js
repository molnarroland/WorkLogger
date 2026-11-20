// @ts-check
const eslint = require('@eslint/js');
const tseslint = require('typescript-eslint');
const angular = require('angular-eslint');

// Layering (see CONTRIBUTING.md):
//   ui/     dumb components: inputs/outputs only. May import api/ for types, never state/ or pages/.
//   shared/ plain functions: no Angular DI, api/ for types only.
//   Only api/wails-api.ts talks to Wails (@wailsio/runtime, @bindings/*).
// Stories and specs may import fixtures from api/, so they are exempt from the api/ value rule.
const wailsOnly = {
  group: ['@wailsio/runtime', '@wailsio/runtime/*', '@bindings/*'],
  message: 'Only src/app/api/wails-api.ts may talk to Wails; go through the Api interface.',
};
const apiValues = {
  group: ['**/api/*', '!**/api/api'],
  message: 'Fixtures and Api implementations are not for production code here; use `import type` from api/api.',
};
const apiTypesOnly = {
  group: ['**/api/api'],
  allowTypeImports: true,
  message: 'Import DTOs with `import type`.',
};

module.exports = tseslint.config(
  {
    ignores: ['dist/', 'bindings/', 'libs/', 'node_modules/', '.angular/', 'storybook-static/', 'coverage/'],
  },
  {
    files: ['**/*.ts'],
    extends: [
      eslint.configs.recommended,
      ...tseslint.configs.recommended,
      ...tseslint.configs.stylistic,
      ...angular.configs.tsRecommended,
    ],
    processor: angular.processInlineTemplates,
    rules: {
      '@angular-eslint/directive-selector': ['error', { type: 'attribute', prefix: 'wl', style: 'camelCase' }],
      '@angular-eslint/component-selector': ['error', { type: 'element', prefix: 'wl', style: 'kebab-case' }],
      '@typescript-eslint/no-unused-vars': ['error', { argsIgnorePattern: '^_', varsIgnorePattern: '^_' }],
      'no-restricted-imports': ['error', { patterns: [wailsOnly] }],
    },
  },
  {
    // wails-api.ts is the Wails adapter; bindings-check.ts only imports the generated types to compare them.
    files: ['src/app/api/wails-api.ts', 'src/app/api/bindings-check.ts'],
    rules: { 'no-restricted-imports': 'off' },
  },
  {
    files: ['src/app/ui/**/*.ts'],
    ignores: ['**/*.stories.ts', '**/*.spec.ts'],
    rules: {
      'no-restricted-imports': [
        'error',
        {
          patterns: [
            wailsOnly,
            apiValues,
            apiTypesOnly,
            { group: ['**/state/*', '**/pages/*'], message: 'ui/ components take inputs and emit outputs only.' },
          ],
        },
      ],
    },
  },
  {
    files: ['src/app/shared/**/*.ts'],
    ignores: ['**/*.stories.ts', '**/*.spec.ts'],
    rules: {
      'no-restricted-imports': [
        'error',
        {
          paths: [{ name: '@angular/core', importNames: ['inject', 'Injectable'], message: 'shared/ holds plain functions, no DI.' }],
          patterns: [wailsOnly, apiValues, apiTypesOnly],
        },
      ],
    },
  },
  {
    files: ['**/*.html'],
    extends: [...angular.configs.templateRecommended, ...angular.configs.templateAccessibility],
    rules: {
      // `snCommand` buttons render their icon and take their accessible name from `[label]`.
      '@angular-eslint/template/elements-content': ['error', { allowList: ['label'] }],
      '@angular-eslint/template/label-has-associated-control': [
        'error',
        { controlComponents: ['hlm-checkbox', 'hlm-switch', 'hlm-select', 'brn-select'] },
      ],
    },
  },
);
