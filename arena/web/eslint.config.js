import tseslint from "typescript-eslint";

export default tseslint.config(
  { ignores: ["dist", "node_modules", "src/protocol/**", "src/generated/**"] },
  ...tseslint.configs.recommended,
  {
    files: ["src/store/**/*.ts"],
    ignores: ["**/*.test.ts"],
    // Layering (PLAN section 4): the store depends only on the protocol.
    rules: {
      "no-restricted-imports": [
        "error",
        {
          patterns: [
            { group: ["**/net/**", "**/render/**", "**/views/**"], message: "store may import only ../protocol" },
          ],
        },
      ],
    },
  },
  {
    files: ["src/render/**/*.ts"],
    ignores: ["**/*.test.ts"],
    rules: {
      "no-restricted-imports": ["error", { patterns: [{ group: ["../**"], message: "render imports only its own files" }] }],
    },
  },
  {
    files: ["src/views/**/*.ts"],
    ignores: ["**/*.test.ts"],
    rules: {
      "no-restricted-imports": [
        "error",
        { patterns: [{ group: ["**/net/**", "**/protocol/**"], message: "views import only store and render" }] },
      ],
    },
  },
);
