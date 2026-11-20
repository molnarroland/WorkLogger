# WorkLogger frontend

The Angular app inside the WorkLogger window. In the app it talks to the Go backend through the generated bindings;
in a plain browser it runs against `FakeApi`, an in-memory backend with a sample month, so most UI work needs no Go
at all. How it fits together: [docs/ARCHITECTURE.md](../docs/ARCHITECTURE.md) and [CONTRIBUTING.md](../CONTRIBUTING.md).

Needs Node 24 (`.nvmrc` at the repository root). After `npm ci`:

| Command | What it does |
| --- | --- |
| `npm start` | The app at http://localhost:4200 with the fake backend |
| `npm run storybook` | Every component in every state, with OS, language and theme switches |
| `npm run test:ci` | Unit tests (Vitest) |
| `npm run test-storybook` | Runs every story's `play` function, with an accessibility check |
| `npm run lint` | ESLint |
| `npm run format` | Prettier (`format:check` only checks) |
| `npm run build` | The production build into `dist/`, which the Go binary embeds |

From the repository root:

| Command | When |
| --- | --- |
| `task bindings` | after changing a Go DTO or service method: regenerates `bindings/` |
| `task fixtures` | after changing a view in `internal/view`: regenerates `src/app/api/testing/sample.json` |
| `task dev` | the whole app with live reload |
