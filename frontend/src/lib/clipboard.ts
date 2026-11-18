import { toast } from "sonner";

/** Copies text and reports the outcome; `what` names the thing copied ("Image ID"). */
export function copyToClipboard(text: string, what: string) {
  navigator.clipboard
    .writeText(text)
    .then(() => toast.success(`${what} copied`))
    .catch(() => toast.error(`Could not copy ${what.toLowerCase()}`));
}
