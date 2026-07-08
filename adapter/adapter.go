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

// Version est la version du plugin affichée dans l'écran Paramètres.
const Version = "V.1.7.0"

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
		// Tableau de bord conforme à la maquette sungrow-plugin-ux (vue 2).
		Dashboard: &plugin.DashboardSpec{
			Cards: []plugin.CardSpec{
				{Label: "Production PV", Icon: "sun", Tone: "solar", Metric: "pv_power",
					Sub: []plugin.SubRef{{Label: "aujourd'hui", Metric: "pv_energy_today"}}},
				{Label: "Consommation", Icon: "home", Tone: "load", Metric: "load_power",
					SubText: "foyer"},
				{Label: "Injection réseau", Icon: "arrow-up", Tone: "battery", ValueTone: "battery", Metric: "grid_export_power",
					Sub: []plugin.SubRef{{Label: "soutirage", Metric: "grid_import_power"}}},
				{Label: "Batterie SBR064", Icon: "battery", Tone: "battery", Metric: "battery_soc",
					Sub: []plugin.SubRef{{Metric: "battery_temp"}, {Label: "santé", Metric: "battery_soh"}}},
			},
			Gauge: &plugin.GaugeSpec{
				Title: "Taux d'autoconsommation", Numerator: "grid_export_today", Denominator: "pv_energy_today",
				Invert: true, Label: "autoconsommé", LegendA: "Autoconsommé", LegendB: "Injecté", Tone: "battery",
			},
			Chart: &plugin.ChartSpec{
				Title: "Production du jour", Metric: "pv_power", Unit: "kW", Tone: "solar",
				Stats: []plugin.StatRef{
					{Label: "Énergie produite", Metric: "pv_energy_today"},
					{Label: "Pic", Peak: true, Tone: "solar"},
					{Label: "Batterie chargée", Metric: "battery_charge_today", Tone: "battery"},
				},
			},
			Flow: &plugin.FlowSpec{
				PV: "pv_power", Load: "load_power",
				GridImport: "grid_import_power", GridExport: "grid_export_power",
				BatteryCharge: "battery_charge_power", BatteryDischarge: "battery_discharge_power",
				BatterySoc: "battery_soc",
			},
		},
		Metrics: []plugin.MetricDisplay{
			{Name: "pv_power", Label: "Production PV", Unit: "kW", Tone: "solar"},
			{Name: "load_power", Label: "Consommation maison", Unit: "kW", Tone: "load"},
			{Name: "grid_export_power", Label: "Injection réseau", Unit: "kW", Tone: "battery"},
			{Name: "grid_import_power", Label: "Soutirage réseau", Unit: "kW", Tone: "grid"},
			{Name: "battery_soc", Label: "État de charge", Unit: "%", Tone: "battery"},
			{Name: "battery_soh", Label: "Santé batterie", Unit: "%"},
			{Name: "battery_temp", Label: "Température batterie", Unit: "°C"},
			{Name: "pv_energy_today", Label: "Production du jour", Unit: "kWh", Tone: "solar"},
			{Name: "grid_export_today", Label: "Injecté aujourd'hui", Unit: "kWh"},
			{Name: "battery_charge_today", Label: "Batterie chargée aujourd'hui", Unit: "kWh", Tone: "battery"},
			{Name: "battery_charge_power", Label: "Charge batterie", Unit: "kW", Tone: "battery"},
			{Name: "battery_discharge_power", Label: "Décharge batterie", Unit: "kW", Tone: "battery"},
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
