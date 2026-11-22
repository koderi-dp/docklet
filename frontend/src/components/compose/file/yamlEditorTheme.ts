import { HighlightStyle, syntaxHighlighting } from "@codemirror/language";
import { EditorView } from "@codemirror/view";
import { tags as t } from "@lezer/highlight";

// Every colour is a CSS variable of the app theme, so dark and light need no rebuild.
const mono = "var(--font-mono)";

export const yamlTheme = EditorView.theme({
  "&": {
    height: "100%",
    backgroundColor: "var(--card)",
    color: "var(--foreground)",
    fontSize: "12.5px",
  },
  "&.cm-focused": { outline: "none" },
  ".cm-scroller": { fontFamily: mono, lineHeight: "1.6", overflow: "auto" },
  ".cm-content": { caretColor: "var(--primary)", padding: "8px 0" },
  ".cm-cursor, .cm-dropCursor": { borderLeftColor: "var(--primary)" },
  ".cm-gutters": {
    backgroundColor: "var(--card)",
    color: "var(--muted-foreground)",
    border: "none",
    borderRight: "1px solid var(--border)",
  },
  ".cm-lineNumbers .cm-gutterElement": { padding: "0 10px 0 12px" },
  ".cm-activeLine": { backgroundColor: "color-mix(in oklab, var(--muted) 55%, transparent)" },
  ".cm-activeLineGutter": { backgroundColor: "var(--muted)", color: "var(--foreground)" },
  "&.cm-focused > .cm-scroller > .cm-selectionLayer .cm-selectionBackground, .cm-selectionBackground, .cm-content ::selection": {
    backgroundColor: "color-mix(in oklab, var(--primary) 28%, transparent)",
  },
  ".cm-selectionMatch": { backgroundColor: "color-mix(in oklab, var(--primary) 16%, transparent)" },
  "&.cm-focused .cm-matchingBracket": { backgroundColor: "color-mix(in oklab, var(--primary) 22%, transparent)", outline: "none" },
  "&.cm-focused .cm-nonmatchingBracket": { backgroundColor: "color-mix(in oklab, var(--destructive) 25%, transparent)" },
  ".cm-foldPlaceholder": { backgroundColor: "var(--muted)", border: "none", color: "var(--muted-foreground)" },
  ".cm-searchMatch": { backgroundColor: "var(--search-match)", outline: "none" },
  ".cm-searchMatch-selected": { backgroundColor: "var(--search-current)", color: "var(--search-current-foreground)" },
  ".cm-panels": { backgroundColor: "var(--card)", color: "var(--foreground)", fontFamily: "var(--font-sans)" },
  ".cm-panels.cm-panels-top": { borderBottom: "1px solid var(--border)" },
  ".cm-panels.cm-panels-bottom": { borderTop: "1px solid var(--border)" },
  ".cm-panel.cm-search": { padding: "6px 28px 6px 8px", fontSize: "12px" },
  ".cm-panel.cm-search label": { color: "var(--muted-foreground)" },
  ".cm-panel.cm-search input, .cm-panel.cm-search button": { fontFamily: "var(--font-sans)", fontSize: "12px" },
  ".cm-panel.cm-search input.cm-textfield": {
    backgroundColor: "var(--input)",
    color: "var(--foreground)",
    border: "1px solid var(--border)",
    borderRadius: "6px",
    padding: "2px 6px",
  },
  ".cm-panel.cm-search input.cm-textfield:focus": { outline: "1px solid var(--ring)" },
  ".cm-panel.cm-search button.cm-button": {
    backgroundImage: "none",
    backgroundColor: "var(--secondary)",
    color: "var(--secondary-foreground)",
    border: "1px solid var(--border)",
    borderRadius: "6px",
    padding: "2px 8px",
  },
  ".cm-panel.cm-search button.cm-button:hover": { backgroundColor: "var(--accent)" },
  ".cm-panel.cm-search [name=close]": { color: "var(--muted-foreground)", backgroundColor: "transparent" },
  ".cm-tooltip": { backgroundColor: "var(--popover)", color: "var(--popover-foreground)", border: "1px solid var(--border)" },
});

export const yamlHighlight = HighlightStyle.define([
  { tag: [t.propertyName, t.definition(t.propertyName)], color: "var(--primary)" },
  { tag: [t.string, t.special(t.string)], color: "var(--status-success)" },
  { tag: [t.number, t.integer, t.float], color: "var(--status-warning)" },
  { tag: [t.bool, t.null, t.atom, t.keyword], color: "var(--status-info)" },
  { tag: [t.labelName, t.typeName, t.meta], color: "var(--status-warning)" },
  { tag: [t.comment, t.lineComment], color: "var(--muted-foreground)", fontStyle: "italic" },
  { tag: [t.punctuation, t.separator, t.squareBracket, t.brace], color: "var(--muted-foreground)" },
]);

export const yamlThemeExtensions = [yamlTheme, syntaxHighlighting(yamlHighlight)];
