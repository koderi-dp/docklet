package buildkit

import "docklet/internal/model"

func modelProfile(typ, endpoint, distro string) model.ConnectionProfile {
	return model.ConnectionProfile{Type: model.ConnectionType(typ), Endpoint: endpoint, WSLDistro: distro}
}
