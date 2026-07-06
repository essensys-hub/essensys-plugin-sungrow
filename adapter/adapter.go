// Package sungrow est l'adaptateur backend du plugin Sungrow. Il traduit les
// messages MQTT du collecteur en échantillons et fournit le descripteur UI.
// Lecture seule : aucune écriture vers l'armoire, aucune référence legacy.
package sungrow

import (
	"encoding/json"
	"strings"
	"time"

	plugin "github.com/essensys-hub/essensys-plugin-framework/go"
)

const ID = "sungrow-solar"

// Adapter implémente plugin.PluginAdapter.
type Adapter struct{}

// New crée l'adaptateur (enregistré dans le registre compilé de l'app hôte).
func New() *Adapter { return &Adapter{} }

func (Adapter) ID() string { return ID }

func (Adapter) Descriptor() plugin.Descriptor {
	return plugin.Descriptor{
		PluginID: ID,
		Title:    "Solaire",
		Tile:     &plugin.TileSpec{Icon: "sun", Primary: "pv_power"},
		Page:     &plugin.PageSpec{Chart: "flow"},
		ReadOnly: true,
		Metrics: []plugin.MetricDisplay{
			{Name: "pv_power", Label: "Production PV", Unit: "kW", Tone: "solar"},
			{Name: "load_power", Label: "Consommation maison", Unit: "kW", Tone: "load"},
			{Name: "grid_export_power", Label: "Injection réseau", Unit: "kW", Tone: "battery"},
			{Name: "grid_import_power", Label: "Soutirage réseau", Unit: "kW", Tone: "grid"},
			{Name: "battery_soc", Label: "État de charge", Unit: "%", Tone: "battery"},
			{Name: "battery_soh", Label: "Santé batterie", Unit: "%"},
			{Name: "battery_temp", Label: "Température batterie", Unit: "°C"},
			{Name: "pv_energy_today", Label: "Production du jour", Unit: "kWh", Tone: "solar"},
		},
	}
}

type payload struct {
	Value float64 `json:"value"`
	Unit  string  `json:"unit"`
	TS    int64   `json:"ts"`
}

// OnMessage traduit un message topic essensys/plugins/sungrow-solar/<m>/<metric>.
func (Adapter) OnMessage(msg plugin.BusMessage) ([]plugin.Sample, error) {
	metric := lastSegment(msg.Topic)
	if metric == "" || strings.HasPrefix(metric, "_") { // _heartbeat et co: ignorés
		return nil, nil
	}
	var p payload
	if err := json.Unmarshal(msg.Payload, &p); err != nil {
		return nil, err
	}
	ts := time.Now()
	if p.TS > 0 {
		ts = time.Unix(p.TS, 0)
	}
	return []plugin.Sample{{
		Metric: metric, Value: p.Value, Unit: p.Unit,
		MachineID: msg.MachineID, TS: ts,
	}}, nil
}

func lastSegment(topic string) string {
	i := strings.LastIndex(topic, "/")
	if i < 0 {
		return topic
	}
	return topic[i+1:]
}
