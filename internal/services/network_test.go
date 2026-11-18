package services

import (
	"context"
	"strings"
	"testing"

	"docklet/internal/model"
)

func TestNetworkRemove_RefusesPredefinedAndInUseNetworks(t *testing.T) {
	f, prov, em := newResFake(t)
	s := &NetworkService{Engines: prov, Events: em}
	res, err := s.Remove(context.Background(), "c1", []string{"bridge", "appnet"})
	if err != nil {
		t.Fatal(err)
	}
	if res[0].OK || !strings.Contains(res[0].Error, "predefined") {
		t.Errorf("bridge: %+v", res[0])
	}
	if f.called("DELETE /networks/n-bridge") {
		t.Error("engine delete must not be attempted for predefined networks")
	}
	if res[1].OK || !strings.Contains(res[1].Error, "in use by web") {
		t.Errorf("appnet: %+v", res[1])
	}
}

func TestNetworkInspect_ListsAttachedContainersAndIPAM(t *testing.T) {
	_, prov, em := newResFake(t)
	s := &NetworkService{Engines: prov, Events: em}
	d, err := s.Inspect(context.Background(), "c1", "appnet")
	if err != nil || len(d.Attached) != 1 || d.Attached[0].IPv4 != "172.20.0.2/16" || d.IPAM[0].Gateway != "172.20.0.1" || len(d.Subnets) != 1 {
		t.Fatalf("inspect: %+v %v", d, err)
	}
}

func TestValidateNetwork_RejectsUnsafeOrInconsistentRequests(t *testing.T) {
	bad := map[string]model.NetworkCreateRequest{
		"empty name":             {Name: ""},
		"predefined name":        {Name: "bridge"},
		"space in name":          {Name: "a b"},
		"gateway without subnet": {Name: "ok", Gateway: "10.0.0.1"},
		"unparsable subnet":      {Name: "ok", Subnet: "nonsense"},
		"gateway outside subnet": {Name: "ok", Subnet: "10.0.0.0/24", Gateway: "10.1.0.1"},
	}
	for name, r := range bad {
		if _, _, _, err := validateNetwork(r); err == nil {
			t.Errorf("%s: want error", name)
		}
	}
}

func TestValidateNetwork_NormalisesSubnetAndDefaultsToBridge(t *testing.T) {
	_, drv, pool, err := validateNetwork(model.NetworkCreateRequest{Name: "good-1", Subnet: "10.5.0.7/24", Gateway: "10.5.0.1"})
	if err != nil || drv != "bridge" || pool.Config[0].Subnet.String() != "10.5.0.0/24" {
		t.Errorf("valid: %v %v %+v", err, drv, pool)
	}
}
