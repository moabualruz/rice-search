## 🧭 Best Practices & Clean Code Guidelines

Maintaining a clean, consistent, and reliable codebase is essential for scalability and collaboration. The following principles should guide all TypeScript contributions.

### 1. General Principles

- Readability over cleverness — Code should be self-explanatory and easy to follow. Future maintainers should understand why something is done, not just what it does.
- Small, focused modules — Keep files and functions short. Each function should do one thing and do it well.
- Avoid repetition (DRY) — Reuse existing utilities or patterns instead of duplicating logic.
- Use meaningful names — Variable, function, and class names should clearly describe their intent and domain.

### 2. TypeScript-Specific Practices

- Prefer explicit types — Always declare return types for exported functions and classes.

```ts
function getUser(id: string): Promise<User> {
  // ...
}
```

- Use interfaces over types for contracts — Especially for public APIs or reusable data structures.
- Enable strict mode — Keep "strict": true in your tsconfig.json to enforce type safety.
- Leverage enums and discriminated unions — Use them for well-defined state management or event typing.
- Avoid any — Replace with generics or unknown where possible to maintain type safety.

### 3. Code Style & Structure

- Follow consistent naming conventions:
  - Classes: PascalCase
  - Variables/functions: camelCase
  - Constants: UPPER_SNAKE_CASE
- Organize imports logically:
  - Built-ins → third-party → internal modules → local files
- Use Biome for formatting and linting. Do not commit code with unresolved lint errors.
- Avoid deeply nested logic — Extract nested logic into helper functions or early returns.
- Document all exported functions with JSDoc — Include parameter descriptions, return types, and behavior notes. Code should be self-documenting through clear naming, but JSDoc provides essential context for public APIs.
- Avoid inline comments unless totally necessary — Prefer self-explanatory code and JSDoc documentation. Inline comments should only explain "why" when the reason is non-obvious, not "what" the code does.
- Separate concerns into focused modules — Group related functionality into dedicated files (e.g., `lib/git.ts` for git operations, `lib/auth.ts` for authentication). Keep modules small and focused on a single responsibility.

### 4. Testing & Validation

- Write unit tests for all core logic and utilities.
- Test-driven mindset — If you fix a bug, add a test that prevents regression.
- Use descriptive test names — Clearly describe expected behavior (e.g., `it('returns 404 if user not found')`).
- Avoid mocking everything — Prefer integration tests where practical.

### 5. Error Handling & Logging

- Fail gracefully — Always handle potential errors with meaningful messages.
- Use custom error classes for domain-specific failures.
- Avoid console logs in production; use structured logging or monitoring utilities.

### 6. Module Organization & Architecture

- Keep utility files focused — Avoid creating monolithic utility files. Split large utility modules by domain (e.g., file operations, git operations, API operations).
- Group related functionality — Place related functions in the same module. Use the `lib/` directory for reusable, domain-specific utilities.
- Export only what's needed — Make functions private (not exported) when they're only used within the same module. Export only the public API.

### 7. Git & Code Review

- Commit small and often — Each commit should have a single purpose.
- Use descriptive commit messages following the `type(scope): description` convention (e.g., `feat(auth): add token refresh logic`).
- Be open to feedback — Reviews are opportunities for shared learning and improving code quality.


