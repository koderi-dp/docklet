package model

// EventContainerStatsData payload: ContainerStatsData.
const EventContainerStatsData = "containerstats:data"

// ContainerStatsData is one batched update of a live stats watch. It carries
// only containers that produced a new sample since the previous batch.
type ContainerStatsData struct {
	WatchID      string          `json:"watchId"`
	ConnectionID string          `json:"connectionId"`
	Stats        []ContainerStat `json:"stats"`
}
