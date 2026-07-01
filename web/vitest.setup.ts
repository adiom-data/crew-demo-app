// Register jest-dom matchers with vitest's expect. We extend explicitly (rather
// than importing "@testing-library/jest-dom/vitest") because this repo uses
// isolated node-linker, and jest-dom's vitest entry cannot resolve vitest from
// the hoisted store — but this setup file can.
import * as matchers from "@testing-library/jest-dom/matchers";
import { expect } from "vitest";

expect.extend(matchers);
