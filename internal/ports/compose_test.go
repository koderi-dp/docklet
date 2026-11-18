package ports

import "testing"

func TestParseComposePorts_ReadsPublishedHostPorts(t *testing.T) {
	cfg := `{"services":{
	 "web":{"ports":[{"mode":"ingress","host_ip":"127.0.0.1","target":80,"published":"8080","protocol":"tcp"},
	               {"target":443,"published":"8443","protocol":"tcp"},
	               {"target":9000,"protocol":"tcp"}]},
	 "dns":{"ports":[{"target":53,"published":"5353","protocol":"udp"},
	               {"target":7000,"published":"7000-7002","protocol":"tcp"}]},
	 "db":{"image":"postgres"}}}`
	all, err := ParseComposePorts([]byte(cfg), "")
	if err != nil {
		t.Fatal(err)
	}
	if len(all) != 6 {
		t.Fatalf("got %+v", all)
	}
	only, _ := ParseComposePorts([]byte(cfg), "web")
	if len(only) != 2 || only[0] != (Request{HostIP: "127.0.0.1", HostPort: 8080, Protocol: "tcp"}) {
		t.Errorf("service filter = %+v", only)
	}
}

func TestParseComposePorts_RejectsGarbageInsteadOfReportingNoPorts(t *testing.T) {
	if _, err := ParseComposePorts([]byte("not json"), ""); err == nil {
		t.Error("expected an error")
	}
}
