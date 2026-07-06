package sungrow

import (
	"testing"

	plugin "github.com/essensys-hub/essensys-plugin-framework/go"
)

func TestOnMessageParsesSample(t *testing.T) {
	a := New()
	msg := plugin.BusMessage{
		Topic:     plugin.Topic(ID, "A254", "pv_power"),
		MachineID: "A254",
		Payload:   []byte(`{"value":5.81,"unit":"kW","ts":1783351102}`),
	}
	got, err := a.OnMessage(msg)
	if err != nil {
		t.Fatalf("OnMessage: %v", err)
	}
	if len(got) != 1 || got[0].Metric != "pv_power" || got[0].Value != 5.81 || got[0].MachineID != "A254" {
		t.Fatalf("échantillon inattendu: %+v", got)
	}
}

func TestHeartbeatIgnored(t *testing.T) {
	a := New()
	msg := plugin.BusMessage{Topic: plugin.HeartbeatTopic(ID, "A254"), Payload: []byte(`{"ts":1}`)}
	got, err := a.OnMessage(msg)
	if err != nil || got != nil {
		t.Fatalf("heartbeat devrait être ignoré, got %+v err %v", got, err)
	}
}

// Le plugin s'intègre au registre du framework et sert /current end-to-end.
func TestEndToEndThroughRegistry(t *testing.T) {
	store := plugin.NewMemStore(60_000_000_000) // 60s
	sink := plugin.NewMemSink()
	reg := plugin.New(store, sink)
	reg.Register(New())

	m := plugin.Manifest{
		ID: ID, ManifestVersion: 1, FrameworkVersion: "^1.0",
		Capabilities: []string{"metrics"}, Perimeters: []plugin.Perimeter{plugin.PerimeterLANCM5},
		Visibility: []plugin.Role{plugin.RoleUser}, WriteScope: "read-only",
	}
	m.Surfaces.Backend = &struct {
		Adapter string `json:"adapter"`
	}{Adapter: ID}
	if err := reg.Configure(m); err != nil {
		t.Fatalf("configure: %v", err)
	}
	reg.Ingest(New(), plugin.BusMessage{
		Topic: plugin.Topic(ID, "A254", "battery_soc"), MachineID: "A254",
		Payload: []byte(`{"value":100,"unit":"%","ts":1783351102}`),
	})
	cur := store.Current(ID)
	if len(cur.Samples) != 1 || cur.Samples[0].Metric != "battery_soc" {
		t.Fatalf("current inattendu: %+v", cur)
	}
}
