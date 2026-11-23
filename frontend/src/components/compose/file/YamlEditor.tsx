import { useEffect, useRef } from "react";
import { basicSetup } from "codemirror";
import { Compartment, EditorState, Prec, type Extension } from "@codemirror/state";
import { EditorView, keymap } from "@codemirror/view";
import { indentLess } from "@codemirror/commands";
import { yaml } from "@codemirror/lang-yaml";
import { yamlThemeExtensions } from "./yamlEditorTheme";

interface Props {
  value: string;
  readOnly: boolean;
  onChange: (text: string) => void;
  onSave: () => void;
  jumpTo?: { line: number; nonce: number };
  /** Suggestions for this editor's language, e.g. composeCompletion(). It may change while the editor lives. */
  completion?: Extension;
}

/** A CodeMirror 6 view that is created once; callbacks and props reach it through refs and effects. */
export function YamlEditor({ value, readOnly, onChange, onSave, jumpTo, completion }: Props) {
  const host = useRef<HTMLDivElement>(null);
  const view = useRef<EditorView | null>(null);
  const extensions = useRef<Extension | null>(null);
  const readOnlyCompartment = useRef(new Compartment());
  const completionCompartment = useRef(new Compartment());
  const latest = useRef({ value, readOnly, onChange, onSave, completion });
  latest.current = { value, readOnly, onChange, onSave, completion };
  /** The parts of the state that follow props: rebuilt whenever a new document replaces the state. */
  const dynamic = () => [
    readOnlyCompartment.current.of(EditorState.readOnly.of(latest.current.readOnly)),
    // Before basicSetup, so these suggestions take the place of its default completion.
    completionCompartment.current.of(latest.current.completion ?? []),
  ];

  useEffect(() => {
    const ext: Extension = [
      Prec.highest(
        keymap.of([
          {
            key: "Mod-s",
            run: () => {
              latest.current.onSave();
              return true;
            },
          },
          {
            key: "Tab",
            run: (v) => {
              if (v.state.readOnly) return false;
              v.dispatch(v.state.replaceSelection("  "), { scrollIntoView: true, userEvent: "input" });
              return true;
            },
            shift: indentLess,
          },
        ]),
      ),
      basicSetup,
      yaml(),
      yamlThemeExtensions,
      EditorView.updateListener.of((u) => {
        if (u.docChanged) latest.current.onChange(u.state.doc.toString());
      }),
    ];
    extensions.current = ext;
    const v = new EditorView({
      state: EditorState.create({
        doc: latest.current.value,
        extensions: [dynamic(), ext],
      }),
      parent: host.current as HTMLElement,
    });
    view.current = v;
    return () => {
      v.destroy();
      view.current = null;
    };
  }, []);

  // A new value from outside (load, reload, file switch) replaces the document, and the undo history with it.
  useEffect(() => {
    const v = view.current;
    if (!v || v.state.doc.toString() === value) return;
    v.setState(
      EditorState.create({
        doc: value,
        extensions: [dynamic(), extensions.current as Extension],
      }),
    );
  }, [value]);

  useEffect(() => {
    view.current?.dispatch({ effects: readOnlyCompartment.current.reconfigure(EditorState.readOnly.of(readOnly)) });
  }, [readOnly]);

  useEffect(() => {
    view.current?.dispatch({ effects: completionCompartment.current.reconfigure(completion ?? []) });
  }, [completion]);

  useEffect(() => {
    const v = view.current;
    if (!v || !jumpTo) return;
    const line = v.state.doc.line(Math.min(Math.max(jumpTo.line, 1), v.state.doc.lines));
    v.dispatch({ selection: { anchor: line.from }, effects: EditorView.scrollIntoView(line.from, { y: "center" }) });
    v.focus();
  }, [jumpTo?.nonce]); // eslint-disable-line react-hooks/exhaustive-deps

  return <div ref={host} className="h-full min-h-0 overflow-hidden" />;
}
