import { cleanup } from "@testing-library/react";
import { afterEach } from "vite-plus/test";

// Testing Library unmounts what a test rendered only when the test functions are globals, and here they're imported.
afterEach(cleanup);
