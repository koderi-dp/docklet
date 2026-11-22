import type { RuleKind, RuleTarget } from "@/lib/logFilter/rules";

export const KIND_LABEL: Record<RuleKind, string> = { include: "Include", exclude: "Exclude", highlight: "Highlight" };
export const TARGET_LABEL: Record<RuleTarget, string> = { message: "Message", container: "Container", level: "Level" };
