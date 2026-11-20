import { describe, expect, it } from "vitest";
import { parseAnsi, type AnsiStyle } from "./ansi";

const E = "\u001b";
/** The style of the single styled range "x" after the given SGR parameters. */
const styleOf = (params: string): AnsiStyle | undefined => parseAnsi(`${E}[${params}mx`).spans?.[0].style;

describe("parseAnsi: lines without styles", () => {
  it("returns a line with no escape character as it is, with no range list", () => {
    const line = parseAnsi("plain [31m text");
    expect(line.text).toBe("plain [31m text");
    expect("spans" in line).toBe(false);
  });
  it("keeps no range list when the codes style nothing", () => {
    expect(parseAnsi(`${E}[0mhello${E}[m`)).toEqual({ text: "hello" });
    expect(parseAnsi(`hello${E}[31m`)).toEqual({ text: "hello" });
  });
});

describe("parseAnsi: SGR codes", () => {
  it("reads the 8 standard and 8 bright foreground colours", () => {
    for (let n = 0; n < 8; n++) {
      expect(styleOf(String(30 + n))).toEqual({ fg: n });
      expect(styleOf(String(90 + n))).toEqual({ fg: n + 8 });
    }
  });
  it("reads the 8 standard and 8 bright background colours", () => {
    for (let n = 0; n < 8; n++) {
      expect(styleOf(String(40 + n))).toEqual({ bg: n });
      expect(styleOf(String(100 + n))).toEqual({ bg: n + 8 });
    }
  });
  it("reads bold, dim, italic, underline and inverse", () => {
    expect(styleOf("1")).toEqual({ bold: true });
    expect(styleOf("2")).toEqual({ dim: true });
    expect(styleOf("3")).toEqual({ italic: true });
    expect(styleOf("4")).toEqual({ underline: true });
    expect(styleOf("7")).toEqual({ inverse: true });
  });
  it("combines the codes of one sequence", () => {
    expect(styleOf("1;4;31;44")).toEqual({ bold: true, underline: true, fg: 1, bg: 4 });
  });
  it("keeps the style across sequences until it is reset", () => {
    const line = parseAnsi(`${E}[1m${E}[32mok${E}[0m done`);
    expect(line.text).toBe("ok done");
    expect(line.spans).toEqual([{ start: 0, end: 2, style: { bold: true, fg: 2 } }]);
  });
  it("maps 256-colour values to a named colour, the colour cube or the grey ramp", () => {
    expect(styleOf("38;5;9")).toEqual({ fg: 9 });
    expect(styleOf("38;5;16")).toEqual({ fg: "rgb(0,0,0)" });
    expect(styleOf("38;5;208")).toEqual({ fg: "rgb(255,135,0)" });
    expect(styleOf("48;5;231")).toEqual({ bg: "rgb(255,255,255)" });
    expect(styleOf("48;5;232")).toEqual({ bg: "rgb(8,8,8)" });
    expect(styleOf("38;5;255")).toEqual({ fg: "rgb(238,238,238)" });
  });
  it("reads truecolor foreground and background, and goes on with the codes after them", () => {
    expect(styleOf("38;2;1;20;255")).toEqual({ fg: "rgb(1,20,255)" });
    expect(styleOf("48;2;10;0;0;1")).toEqual({ bg: "rgb(10,0,0)", bold: true });
    expect(styleOf("38;5;208;4")).toEqual({ fg: "rgb(255,135,0)", underline: true });
  });
  it("reads the colon form of the extended colours", () => {
    expect(styleOf("38:5:208")).toEqual({ fg: "rgb(255,135,0)" });
    expect(styleOf("38:2::1:2:3")).toEqual({ fg: "rgb(1,2,3)" });
    expect(styleOf("48:2:1:2:3;1")).toEqual({ bg: "rgb(1,2,3)", bold: true });
  });
  it("ignores an extended colour with missing or out-of-range values", () => {
    expect(parseAnsi(`${E}[38;5mx`)).toEqual({ text: "x" });
    expect(parseAnsi(`${E}[38;5;300mx`)).toEqual({ text: "x" });
    expect(parseAnsi(`${E}[38;2;1;2mx`)).toEqual({ text: "x" });
    expect(parseAnsi(`${E}[48;2;1;2;999mx`)).toEqual({ text: "x" });
  });
  it("ignores codes it does not draw (blink, strikethrough) and keeps the others", () => {
    expect(styleOf("5;9;31")).toEqual({ fg: 1 });
  });
});

describe("parseAnsi: resets", () => {
  const after = (on: string, off: string) => parseAnsi(`${E}[${on}ma${E}[${off}mb`);
  it("0 and the empty sequence clear everything", () => {
    expect(after("1;3;4;7;31;41", "0").spans).toEqual([{ start: 0, end: 1, style: expect.anything() }]);
    expect(after("1;31", "").spans).toHaveLength(1);
  });
  it("22 clears bold and dim, 23 italic, 24 underline, 27 inverse", () => {
    expect(after("1;2;3", "22").spans?.[1].style).toEqual({ italic: true });
    expect(after("3;1", "23").spans?.[1].style).toEqual({ bold: true });
    expect(after("4;1", "24").spans?.[1].style).toEqual({ bold: true });
    expect(after("7;1", "27").spans?.[1].style).toEqual({ bold: true });
  });
  it("39 clears the foreground and 49 the background only", () => {
    expect(after("31;42", "39").spans?.[1].style).toEqual({ bg: 2 });
    expect(after("31;42", "49").spans?.[1].style).toEqual({ fg: 1 });
  });
  it("a reset in the middle of a sequence clears only what came before it", () => {
    expect(styleOf("1;31;0;4")).toEqual({ underline: true });
  });
});

describe("parseAnsi: ranges", () => {
  it("places the ranges on the plain text", () => {
    const line = parseAnsi(`GET ${E}[32m200${E}[0m in ${E}[2m12ms${E}[0m`);
    expect(line.text).toBe("GET 200 in 12ms");
    expect(line.spans).toEqual([
      { start: 4, end: 7, style: { fg: 2 } },
      { start: 11, end: 15, style: { dim: true } },
    ]);
  });
  it("makes one range of styled text that a non-SGR sequence interrupts", () => {
    const line = parseAnsi(`${E}[31mab${E}[Kcd`);
    expect(line).toEqual({ text: "abcd", spans: [{ start: 0, end: 4, style: { fg: 1 } }] });
  });
});

describe("parseAnsi: sequences that are not SGR", () => {
  it("removes cursor movement, erase and private-mode sequences", () => {
    expect(parseAnsi(`${E}[2K${E}[1Gdone${E}[?25h${E}[10;20H`)).toEqual({ text: "done" });
  });
  it("does not take a CSI with a private prefix or an intermediate byte for SGR", () => {
    expect(parseAnsi(`${E}[?31mx`)).toEqual({ text: "x" });
    expect(parseAnsi(`${E}[>4;2mx`)).toEqual({ text: "x" });
    expect(parseAnsi(`${E}[1 mx`)).toEqual({ text: "x" });
  });
  it("removes an OSC title ended by BEL or by ESC \\", () => {
    expect(parseAnsi(`${E}]0;my title\u0007ready`)).toEqual({ text: "ready" });
    expect(parseAnsi(`${E}]2;my title${E}\\ready`)).toEqual({ text: "ready" });
  });
  it("keeps only the label of an OSC 8 hyperlink", () => {
    expect(parseAnsi(`see ${E}]8;;https://example.com${E}\\docs${E}]8;;${E}\\ now`)).toEqual({ text: "see docs now" });
  });
  it("removes charset and two-character sequences", () => {
    expect(parseAnsi(`${E}(Bab${E}=c${E}c`)).toEqual({ text: "abc" });
  });
});

describe("parseAnsi: unterminated and malformed sequences", () => {
  it("drops a CSI cut off by the end of the line", () => {
    expect(parseAnsi(`ok${E}[31`)).toEqual({ text: "ok" });
    expect(parseAnsi(`ok${E}[`)).toEqual({ text: "ok" });
  });
  it("drops a lone escape character", () => {
    expect(parseAnsi(`a${E}`)).toEqual({ text: "a" });
    expect(parseAnsi(`a${E}${E}[31mb`)).toEqual({ text: "ab", spans: [{ start: 1, end: 2, style: { fg: 1 } }] });
  });
  it("drops an unterminated OSC to the end of the line", () => {
    expect(parseAnsi(`a${E}]0;never ends`)).toEqual({ text: "a" });
  });
  it("keeps the text after a CSI whose final byte is not valid", () => {
    expect(parseAnsi(`a${E}[31\u0001b`).text).toBe("a\u0001b");
  });
  it("never leaves an escape character in the text", () => {
    const nasty = `${E}[1;${E}[31mred${E}]8;;x${E}[0m${E}\\${E}`;
    expect(parseAnsi(nasty).text).not.toContain(E);
  });
});
