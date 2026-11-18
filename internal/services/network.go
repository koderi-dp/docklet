package services

import (
	"context"
	"fmt"
	"net/netip"
	"regexp"
	"sort"
	"strings"

	"github.com/moby/moby/api/types/network"
	"github.com/moby/moby/client"

	"docklet/internal/engine"
	"docklet/internal/model"
	"docklet/internal/ops"
)

// NetworkService is bound to the frontend. All methods are scoped by connectionID.
type NetworkService struct {
	Engines engine.Provider
	Events  engine.Emitter
	Ops     *ops.Manager
}

// KindNetworkPrune is the operation kind of Prune Unused on the Networks page.
const KindNetworkPrune = "network.prune"

var networkNameRe = regexp.MustCompile(`^[a-zA-Z0-9][a-zA-Z0-9_.-]+$`)

// predefinedNetworks cannot be removed; Docker itself would refuse, we refuse first with a clear message.
var predefinedNetworks = map[string]bool{"bridge": true, "host": true, "none": true, "nat": true, "ingress": true}

func isPredefinedNetwork(name string) bool { return predefinedNetworks[name] }

func mapNetwork(connectionID string, n network.Network, attached []model.ResourceUser) model.Network {
	subnets, gateways := []string{}, []string{}
	for _, c := range n.IPAM.Config {
		if c.Subnet.IsValid() {
			subnets = append(subnets, c.Subnet.String())
		}
		if c.Gateway.IsValid() {
			gateways = append(gateways, c.Gateway.String())
		}
	}
	labels := n.Labels
	if labels == nil {
		labels = map[string]string{}
	}
	if attached == nil {
		attached = []model.ResourceUser{}
	}
	created := ""
	if !n.Created.IsZero() {
		created = n.Created.Format("2006-01-02T15:04:05Z07:00")
	}
	return model.Network{
		ConnectionID: connectionID, ID: n.ID, Name: n.Name, Driver: n.Driver, Scope: n.Scope,
		Internal: n.Internal, Attachable: n.Attachable, EnableIPv6: n.EnableIPv6,
		Predefined: isPredefinedNetwork(n.Name), Created: created,
		Subnets: subnets, Gateways: gateways, Labels: labels, Attached: attached,
	}
}

// networkUsers maps network ID -> containers whose settings reference it (matched by
// ID, falling back to the network name key).
func networkUsers(ctx context.Context, cli *client.Client) (byID, byName map[string][]model.ResourceUser, err error) {
	res, err := cli.ContainerList(ctx, client.ContainerListOptions{All: true})
	if err != nil {
		return nil, nil, friendlyError(err)
	}
	byID, byName = map[string][]model.ResourceUser{}, map[string][]model.ResourceUser{}
	for _, c := range res.Items {
		if c.NetworkSettings == nil {
			continue
		}
		for netName, ep := range c.NetworkSettings.Networks {
			u := model.ResourceUser{ID: c.ID, Name: containerName(c.Names, c.ID), State: string(c.State)}
			if ep != nil {
				if ep.IPAddress.IsValid() {
					u.IPv4 = ep.IPAddress.String()
				}
				if ep.GlobalIPv6Address.IsValid() {
					u.IPv6 = ep.GlobalIPv6Address.String()
				}
				if ep.NetworkID != "" {
					byID[ep.NetworkID] = append(byID[ep.NetworkID], u)
					continue
				}
			}
			byName[netName] = append(byName[netName], u)
		}
	}
	return byID, byName, nil
}

// List returns all networks with the containers attached to each.
func (s *NetworkService) List(ctx context.Context, connectionID string) ([]model.Network, error) {
	ctx, cancel := callCtx(ctx)
	defer cancel()
	cli, err := engineClient(ctx, s.Engines, connectionID)
	if err != nil {
		return nil, err
	}
	res, err := cli.NetworkList(ctx, client.NetworkListOptions{})
	if err != nil {
		return nil, friendlyError(err)
	}
	byID, byName, err := networkUsers(ctx, cli)
	if err != nil {
		return nil, err
	}
	out := make([]model.Network, 0, len(res.Items))
	for _, n := range res.Items {
		att := append(append([]model.ResourceUser{}, byID[n.ID]...), byName[n.Name]...)
		sort.Slice(att, func(i, j int) bool { return att[i].Name < att[j].Name })
		out = append(out, mapNetwork(connectionID, n.Network, att))
	}
	sort.Slice(out, func(i, j int) bool { return out[i].Name < out[j].Name })
	return out, nil
}

// Inspect returns the Engine's own view of one network (IPAM, options, endpoints).
func (s *NetworkService) Inspect(ctx context.Context, connectionID, idOrName string) (model.NetworkDetail, error) {
	ctx, cancel := callCtx(ctx)
	defer cancel()
	cli, err := engineClient(ctx, s.Engines, connectionID)
	if err != nil {
		return model.NetworkDetail{}, err
	}
	res, err := cli.NetworkInspect(ctx, idOrName, client.NetworkInspectOptions{})
	if err != nil {
		return model.NetworkDetail{}, friendlyNetworkError(err, idOrName, nil)
	}
	return mapNetworkDetail(connectionID, res.Network), nil
}

func mapNetworkDetail(connectionID string, in network.Inspect) model.NetworkDetail {
	att := make([]model.ResourceUser, 0, len(in.Containers))
	for id, ep := range in.Containers {
		u := model.ResourceUser{ID: id, Name: ep.Name}
		if ep.IPv4Address.IsValid() {
			u.IPv4 = ep.IPv4Address.String()
		}
		if ep.IPv6Address.IsValid() {
			u.IPv6 = ep.IPv6Address.String()
		}
		att = append(att, u)
	}
	sort.Slice(att, func(i, j int) bool { return att[i].Name < att[j].Name })
	d := model.NetworkDetail{
		Network:    mapNetwork(connectionID, in.Network, att),
		IPAMDriver: in.IPAM.Driver,
		IPAM:       []model.IPAMConfig{},
		Options:    in.Options,
	}
	if d.Options == nil {
		d.Options = map[string]string{}
	}
	for _, c := range in.IPAM.Config {
		ic := model.IPAMConfig{}
		if c.Subnet.IsValid() {
			ic.Subnet = c.Subnet.String()
		}
		if c.Gateway.IsValid() {
			ic.Gateway = c.Gateway.String()
		}
		if c.IPRange.IsValid() {
			ic.IPRange = c.IPRange.String()
		}
		d.IPAM = append(d.IPAM, ic)
	}
	return d
}

// validateNetwork checks a create request and returns the normalised pieces.
func validateNetwork(req model.NetworkCreateRequest) (name, driver string, pool *network.IPAM, err error) {
	name = strings.TrimSpace(req.Name)
	if name == "" {
		return "", "", nil, fmt.Errorf("Network name is required")
	}
	if !networkNameRe.MatchString(name) {
		return "", "", nil, fmt.Errorf("Invalid network name %q: use at least 2 characters from letters, digits, '_', '.' and '-', starting with a letter or digit", name)
	}
	if isPredefinedNetwork(name) {
		return "", "", nil, fmt.Errorf("%q is a reserved network name", name)
	}
	driver = strings.TrimSpace(req.Driver)
	if driver == "" {
		driver = "bridge"
	}
	if strings.ContainsAny(driver, " \t\r\n/") {
		return "", "", nil, fmt.Errorf("Invalid network driver %q", driver)
	}
	for k := range req.Labels {
		if strings.TrimSpace(k) == "" {
			return "", "", nil, fmt.Errorf("Label keys must not be empty")
		}
	}
	subnet, gw := strings.TrimSpace(req.Subnet), strings.TrimSpace(req.Gateway)
	if subnet == "" && gw == "" {
		return name, driver, nil, nil
	}
	if subnet == "" {
		return "", "", nil, fmt.Errorf("A gateway requires a subnet")
	}
	p, perr := netip.ParsePrefix(subnet)
	if perr != nil {
		return "", "", nil, fmt.Errorf("Invalid subnet %q: expected CIDR such as 172.30.0.0/16", subnet)
	}
	cfg := network.IPAMConfig{Subnet: p.Masked()}
	if gw != "" {
		g, gerr := netip.ParseAddr(gw)
		if gerr != nil {
			return "", "", nil, fmt.Errorf("Invalid gateway %q", gw)
		}
		if !p.Contains(g) {
			return "", "", nil, fmt.Errorf("Gateway %s is not inside subnet %s", g, p.Masked())
		}
		cfg.Gateway = g
	}
	return name, driver, &network.IPAM{Config: []network.IPAMConfig{cfg}}, nil
}

// Create makes a network after validating name, driver, subnet and gateway.
func (s *NetworkService) Create(ctx context.Context, connectionID string, req model.NetworkCreateRequest) (model.NetworkDetail, error) {
	name, driver, pool, err := validateNetwork(req)
	if err != nil {
		return model.NetworkDetail{}, err
	}
	cli, err := engineClient(ctx, s.Engines, connectionID)
	if err != nil {
		return model.NetworkDetail{}, err
	}
	res, err := cli.NetworkCreate(ctx, name, client.NetworkCreateOptions{
		Driver: driver, IPAM: pool, Internal: req.Internal, Attachable: req.Attachable, Labels: req.Labels,
	})
	if err != nil {
		return model.NetworkDetail{}, friendlyNetworkError(err, name, nil)
	}
	emit(s.Events, model.EventNetworksChanged, model.NetworksChanged{ConnectionID: connectionID})
	ins, err := cli.NetworkInspect(ctx, res.ID, client.NetworkInspectOptions{})
	if err != nil {
		// Created but not readable back: report what we know instead of failing the create.
		return model.NetworkDetail{Network: model.Network{ConnectionID: connectionID, ID: res.ID, Name: name, Driver: driver}}, nil
	}
	return mapNetworkDetail(connectionID, ins.Network), nil
}

// Remove deletes networks (IDs or names). Predefined networks are refused per item.
func (s *NetworkService) Remove(ctx context.Context, connectionID string, idsOrNames []string) ([]model.ActionResult, error) {
	cli, err := engineClient(ctx, s.Engines, connectionID)
	if err != nil {
		return nil, err
	}
	results := make([]model.ActionResult, 0, len(idsOrNames))
	anyOK := false
	for _, in := range idsOrNames {
		r := model.ActionResult{ID: in, Name: in}
		ins, err := cli.NetworkInspect(ctx, in, client.NetworkInspectOptions{})
		if err != nil {
			r.Error = friendlyNetworkError(err, in, nil).Error()
			results = append(results, r)
			continue
		}
		r.ID, r.Name = ins.Network.ID, ins.Network.Name
		if isPredefinedNetwork(ins.Network.Name) {
			r.Error = fmt.Sprintf("%q is a predefined Docker network and cannot be removed", ins.Network.Name)
			results = append(results, r)
			continue
		}
		if _, err := cli.NetworkRemove(ctx, ins.Network.ID, client.NetworkRemoveOptions{}); err != nil {
			r.Error = friendlyNetworkError(err, ins.Network.Name, mapNetworkDetail(connectionID, ins.Network).Attached).Error()
		} else {
			r.OK = true
			anyOK = true
		}
		results = append(results, r)
	}
	if anyOK {
		emit(s.Events, model.EventNetworksChanged, model.NetworksChanged{ConnectionID: connectionID})
	}
	return results, nil
}

// Connect attaches a container to a network.
func (s *NetworkService) Connect(ctx context.Context, connectionID, networkID, containerID string) error {
	cli, err := engineClient(ctx, s.Engines, connectionID)
	if err != nil {
		return err
	}
	if _, err := cli.NetworkConnect(ctx, networkID, client.NetworkConnectOptions{Container: containerID}); err != nil {
		return friendlyNetworkError(err, networkID, nil)
	}
	emit(s.Events, model.EventNetworksChanged, model.NetworksChanged{ConnectionID: connectionID})
	emit(s.Events, model.EventContainersChanged, model.ContainersChanged{ConnectionID: connectionID})
	return nil
}

// Disconnect detaches a container from a network (never forced).
func (s *NetworkService) Disconnect(ctx context.Context, connectionID, networkID, containerID string) error {
	cli, err := engineClient(ctx, s.Engines, connectionID)
	if err != nil {
		return err
	}
	if _, err := cli.NetworkDisconnect(ctx, networkID, client.NetworkDisconnectOptions{Container: containerID}); err != nil {
		return friendlyNetworkError(err, networkID, nil)
	}
	emit(s.Events, model.EventNetworksChanged, model.NetworksChanged{ConnectionID: connectionID})
	emit(s.Events, model.EventContainersChanged, model.ContainersChanged{ConnectionID: connectionID})
	return nil
}

// PruneUnused removes networks no container uses (the Engine never prunes predefined ones).
func (s *NetworkService) PruneUnused(ctx context.Context, connectionID string) (model.Operation, error) {
	return startPrune(ctx, s.Engines, s.Ops, s.Events, connectionID,
		pruneSpec{Kind: KindNetworkPrune, Title: "Prune unused networks", Resource: "networks", Changed: []string{model.EventNetworksChanged}},
		pruneNetworksStep)
}

func pruneNetworksStep(ctx context.Context, cli *client.Client) (pruneOutcome, error) {
	res, err := cli.NetworkPrune(ctx, client.NetworkPruneOptions{})
	if err != nil {
		return pruneOutcome{}, err
	}
	return pruneOutcome{Noun: "networks", Items: res.Report.NetworksDeleted}, nil
}

func friendlyNetworkError(err error, name string, users []model.ResourceUser) error {
	low := strings.ToLower(err.Error())
	switch {
	case strings.Contains(low, "pre-defined") || strings.Contains(low, "predefined"):
		return fmt.Errorf("%q is a predefined Docker network and cannot be removed", name)
	case strings.Contains(low, "already exists") || strings.Contains(low, "name conflicts"):
		return fmt.Errorf("A network named %q already exists", name)
	case strings.Contains(low, "pool overlaps") || strings.Contains(low, "overlap"):
		return fmt.Errorf("The subnet overlaps an existing network: %w", err)
	case strings.Contains(low, "active endpoints") || strings.Contains(low, "in use"):
		if len(users) > 0 {
			return fmt.Errorf("Network %q is in use by %s", name, userNames(users))
		}
		return fmt.Errorf("Network %q is in use by a container: %w", name, err)
	case strings.Contains(low, "plugin") && strings.Contains(low, "not found"):
		return fmt.Errorf("The network driver is not available on this Engine: %s", err.Error())
	case isNotFound(err):
		return fmt.Errorf("Network or container not found (it may have been removed): %s", err.Error())
	}
	return friendlyError(err)
}
