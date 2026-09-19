import assert from "node:assert/strict";
import test from "node:test";
import { maskInlineCode } from "./markdown-inline-code.mjs";

test("generic calls in code are not Markdown links", () => {
  const code = "`len(copyArray[A](values))`";
  const input = "Use " + code + " and [the guide](guide).";
  assert.equal(maskInlineCode(input), "Use " + " ".repeat(code.length) + " and [the guide](guide).");
});

test("matching runs, multiple spans and unmatched delimiters", () => {
  for (const code of ["`[A](values)`", "``[A](`values`)``", "```[A](``values``)```"]) {
    assert.equal(maskInlineCode(code), " ".repeat(code.length));
  }
  assert.equal(maskInlineCode("`a` + `b`"), "    +    ");
  assert.equal(maskInlineCode("`unclosed [link](page)"), "`unclosed [link](page)");
  assert.equal(maskInlineCode("[link](page)"), "[link](page)");
  assert.equal(maskInlineCode("[`type`](page)"), "[      ](page)");
  assert.equal(maskInlineCode("\\`[link](page)\\`"), "\\`[link](page)\\`");
  assert.equal(maskInlineCode("\\\\`code`"), "\\\\      ");
});
