import { configure } from "@testing-library/react";

// waitFor's own default (1s) is separate from vitest's testTimeout and
// expires first under worker starvation on a loaded 4-core box — async
// tests then fail while the suite is merely slow (same rationale as the
// 10s testTimeout in vite.config.ts). 5s still catches a hung callback.
configure({ asyncUtilTimeout: 5000 });
