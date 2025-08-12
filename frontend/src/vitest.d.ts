/// <reference types="vitest" />
/// <reference types="@testing-library/jest-dom" />

interface CustomMatchers<R = unknown> {
  toBeInTheDocument(): R;
}
