package model

// ExternalTerminal selects the terminal used by ExecService.OpenExternalTerminal.
// Mode is "auto" (detect per OS) or "custom"; a custom Template is a command line that
// must contain the {cmd} placeholder, which is replaced by the command to run
// (for example `alacritty -e {cmd}`).
type ExternalTerminal struct {
	Mode     string `json:"mode"`
	Template string `json:"template"`
}
