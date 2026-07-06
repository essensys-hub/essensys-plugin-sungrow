# essensys-plugin-sungrow

Premier plugin Essensys : intégration d'une centrale solaire **Sungrow SH6.0RS** + batterie **SBR064**, en lecture seule.

## Source de données
API locale du dongle **WiNet-S** : `wss://<ip>/ws/home/overview` (login local `user`/`pw1111` via SOPS). Aucun passage par le cloud iSolarCloud ; le protocole legacy IoT n'est pas touché.

## Métriques exposées
Production PV, consommation foyer, injection/soutirage réseau, SoC / santé / température batterie, énergies journalières et totales.

## Briques
- `collector/` — collecteur MQTT dérivé de `sungrow_winet_collector.py`.
- `adapter/` — adaptateur Go (`/api/plugins/sungrow-solar/*`).
- `ui/` — tuile « Solaire » (flux instantané) + page détail (historique Prometheus).

## Périmètres
Plugin **device-LAN** : collecteur sur CM5/gateway ; cloud via cloudsync ; **indisponible** en « armoire seule WAN ».

Construit sur [`essensys-plugin-framework`](https://github.com/essensys-hub/essensys-plugin-framework).
