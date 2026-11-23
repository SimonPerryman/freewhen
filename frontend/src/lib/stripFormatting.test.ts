import { describe, it, expect } from "vitest";
import { stripFormatting } from "./stripFormatting";

describe("stripFormatting", () => {
  it.each([
    ["**Board games**", "Board games"],
    ["*Friday* drinks", "Friday drinks"],
    ["__Book__ club", "Book club"],
    ["_Quiet_ night", "Quiet night"],
    ["~~Old~~ plan", "Old plan"],
    ["`code` jam", "code jam"],
    ["# Heading", "Heading"],
    ["[Site](https://x.test) meetup", "Site meetup"],
    ["snake_case_name", "snake_case_name"],
    ["5 * 3 pizzas", "5 * 3 pizzas"],
    ["Plain name", "Plain name"],
  ])("%s -> %s", (input, expected) => {
    expect(stripFormatting(input)).toBe(expected);
  });
});
