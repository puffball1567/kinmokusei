// Mask inline code before scanning a Markdown line for links. Backtick runs
// must match in length; an unmatched opener remains ordinary Markdown text.
export function maskInlineCode(line) {
  const runs = [...line.matchAll(/`+/g)];
  let output = "";
  let start = 0;
  for (let i = 0; i < runs.length; i++) {
    const opener = runs[i];
    let escapes = 0;
    for (let index = opener.index - 1; index >= 0 && line[index] === "\\"; index--) escapes++;
    if (escapes % 2 !== 0) continue;
    let closing = i + 1;
    while (closing < runs.length && runs[closing][0].length !== opener[0].length) closing++;
    if (closing === runs.length) continue;
    const end = runs[closing].index + runs[closing][0].length;
    output += line.slice(start, opener.index) + " ".repeat(end - opener.index);
    start = end;
    i = closing;
  }
  return output + line.slice(start);
}
