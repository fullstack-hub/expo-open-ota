module.exports = {
  root: true,
  extends: ['universe/node'],
  plugins: ['node'],
  ignorePatterns: ['bin/'],
  rules: {
    'no-console': 'warn',
    'no-constant-condition': ['warn', { checkLoops: false }],
    'sort-imports': [
      'warn',
      {
        ignoreDeclarationSort: true,
      },
    ],
    curly: 'warn',
    'import/no-cycle': 'error',
    'import/no-extraneous-dependencies': [
      'error',
      { devDependencies: ['**/__tests__/**/*', '**/__mocks__/**/*'] },
    ],
    'import/no-relative-packages': 'error',
    'no-restricted-imports': [
      'error',
      {
        paths: [
          {
            name: 'lodash',
            message: "Don't use lodash, it's heavy!",
          },
        ],
      },
    ],
    'no-underscore-dangle': ['error', { allow: ['__typename'] }],
    'node/no-sync': 'error',
  },
  overrides: [
    {
      files: ['*.ts', '*.d.ts'],
      parserOptions: {
        project: './tsconfig.eslint.json',
      },
      rules: {
        '@typescript-eslint/explicit-function-return-type': [
          'warn',
          {
            allowExpressions: true,
          },
        ],
        '@typescript-eslint/prefer-nullish-coalescing': ['warn', { ignorePrimitives: true }],
        '@typescript-eslint/no-confusing-void-expression': 'warn',
        '@typescript-eslint/await-thenable': 'error',
        '@typescript-eslint/no-misused-promises': [
          'error',
          {
            checksVoidReturn: false,
          },
        ],
        '@typescript-eslint/no-floating-promises': 'error',
        'no-void': ['warn', { allowAsStatement: true }],
        'no-return-await': 'off',
        '@typescript-eslint/return-await': ['error', 'always'],
        '@typescript-eslint/no-confusing-non-null-assertion': 'warn',
        '@typescript-eslint/no-extra-non-null-assertion': 'warn',
        '@typescript-eslint/prefer-as-const': 'warn',
        '@typescript-eslint/prefer-includes': 'warn',
        '@typescript-eslint/prefer-readonly': 'warn',
        '@typescript-eslint/prefer-string-starts-ends-with': 'warn',
        '@typescript-eslint/prefer-ts-expect-error': 'warn',
        '@typescript-eslint/no-unnecessary-type-assertion': 'warn',
      },
    },
    {
      // node:test 의 describe/it 는 Promise 를 반환하지만 최상위에서 await 하지
      // 않는 게 정상적인 사용 패턴이라, 테스트 파일에서는 floating-promises 를 끈다.
      files: ['test/**/*.ts'],
      rules: {
        '@typescript-eslint/no-floating-promises': 'off',
      },
    },
  ],
};
