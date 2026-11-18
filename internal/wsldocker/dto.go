package wsldocker

import "docklet/internal/model"

// DTO converts the plan to the shape sent to the UI. SystemdSteps are
// returned separately so the UI can show them only when the option is ticked.
func (p Plan) DTO() model.WSLInstallPlan {
	f := p.Facts
	user := f.User
	if user == "root" || !unixUser.MatchString(user) {
		user = ""
	}
	out := model.WSLInstallPlan{
		Distro: f.Distro, DistroPretty: f.OS.PrettyName, State: p.State, Supported: p.Supported,
		Reason: p.Reason, DocsURL: p.DocsURL, User: user, Systemd: f.Systemd, DockerDesktop: f.DesktopStub, DaemonStart: p.DaemonStart,
		Steps: stepsDTO(p.Steps), SystemdSteps: stepsDTO(p.SystemdSteps),
		PackagesToRemove: p.Remove, Notes: p.Notes,
	}
	if out.Notes == nil {
		out.Notes = []string{}
	}
	return out
}

func stepsDTO(steps []Step) []model.WSLInstallStep {
	out := make([]model.WSLInstallStep, 0, len(steps))
	for _, s := range steps {
		d := model.WSLInstallStep{Title: s.Title, Why: s.Why, Command: Display(s.Cmd), Input: s.Cmd.Stdin}
		if s.Edit != nil {
			d.Command, d.Input = "edit "+s.Edit.Path, s.Edit.Preview
		}
		out = append(out, d)
	}
	return out
}
