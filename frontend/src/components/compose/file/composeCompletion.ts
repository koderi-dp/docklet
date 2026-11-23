import { autocompletion, type Completion, type CompletionContext, type CompletionResult } from "@codemirror/autocomplete";
import type { Extension } from "@codemirror/state";
import { cursorContext } from "@/lib/composeComplete/cursorPath";
import { documentNames, projectValues } from "@/lib/composeComplete/projectValues";
import { keysAt, valuesAt, type Suggestion } from "@/lib/composeComplete/schema";

/** Typing more of a name narrows the list without asking again. */
const VALID_FOR = /^[\w.:/-]*$/;

/** Names from the project rank above what the schema lists. */
const PROJECT_BOOST = 10;

/**
 * Compose suggestions for the cursor position: keys and fixed values from the Compose schema,
 * plus names from the open file and the engine's images. `images` is read on every call, so the
 * list can change without rebuilding the editor.
 */
export function composeSource(images: () => readonly string[]): (ctx: CompletionContext) => CompletionResult | null {
  return (ctx) => {
    const doc = ctx.state.doc;
    const text = doc.toString();
    const at = cursorContext(text, ctx.pos, (n) => (n >= 1 && n <= doc.lines ? doc.line(n).text : undefined));
    if (!at) return null;
    // An empty key position stays quiet until a letter is typed or the list is asked for.
    if (at.kind === "key" && at.prefix === "" && !ctx.explicit) return null;

    const project = projectValues(at, documentNames(text), images());
    const seen = new Set<string>(at.kind === "key" ? at.siblings : []);
    const options: Completion[] = [];
    const add = (list: readonly Suggestion[], type: string, boost: number, apply?: (label: string) => string) => {
      for (const s of list) {
        if (seen.has(s.label)) continue;
        seen.add(s.label);
        options.push({ label: s.label, detail: s.detail, info: s.info, type, boost, apply: apply?.(s.label) });
      }
    };

    if (at.kind === "key") {
      const line = doc.lineAt(ctx.pos);
      const colonFollows = /^[\w.$-]*\s*:/.test(doc.sliceString(ctx.pos, line.to));
      const withColon = colonFollows ? (label: string) => label : (label: string) => `${label}: `;
      add(project, "variable", PROJECT_BOOST, withColon);
      add(keysAt(at.path), "property", 0, withColon);
    } else {
      add(project, "variable", PROJECT_BOOST);
      add(valuesAt(at.path, at.key), "constant", 0);
    }
    if (options.length === 0) return null;
    return { from: at.from, options, validFor: VALID_FOR };
  };
}

/** The editor extension: Compose suggestions replace the default word completion. */
export function composeCompletion(images: () => readonly string[]): Extension {
  return autocompletion({ override: [composeSource(images)], activateOnTyping: true });
}
