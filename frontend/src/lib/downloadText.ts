import { toast } from "sonner";

/** Saves text as a file through the browser download (the same mechanism the log viewer always used). */
export function downloadText(filename: string, text: string): void {
  const blob = new Blob([text], { type: "text/plain" });
  const url = URL.createObjectURL(blob);
  const a = document.createElement("a");
  a.href = url;
  a.download = filename;
  document.body.appendChild(a);
  a.click();
  a.remove();
  setTimeout(() => URL.revokeObjectURL(url), 1000);
  // The web view saves without asking, so say what happened and where to look.
  toast.success(`Saved ${filename}`, { description: "It is in your Downloads folder." });
}
