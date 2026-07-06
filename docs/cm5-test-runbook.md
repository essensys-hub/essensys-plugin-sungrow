# Test du plugin Sungrow sur la CM5 (périmètre LAN)

Objectif : valider la chaîne complète **onduleur → collecteur → MQTT → backend → API → tuile frontend** sur la CM5, avant tout développement pour OVH.

Branches en jeu (non fusionnées) :
- `essensys-server-backend` : `feat/plugin-framework-integration`
- `essensys-server-frontend` : `feat/plugin-sungrow-tile`
- plugins : `main` de `essensys-plugin-framework`, `essensys-plugin-sungrow`

## 0. Prérequis sur la CM5

- Mosquitto, Redis, Prometheus déjà déployés (stack LAN existante).
- Onduleur WiNet-S joignable : `192.168.1.247` (même LAN que la CM5).
- Identifiants WiNet résolus depuis **SOPS** (ne jamais les écrire en clair) :
  `WINET_USER`, `WINET_PASS`.
- Python 3 + `paho-mqtt` pour le collecteur : `pip install -r collector/requirements.txt`.

## 1. Build

### Backend (option rapide : binaire ARM64 cross-compilé depuis le poste dev)
Les 3 dépôts plugins doivent être des **siblings** (layout `~/ESSENSYS/`) car le
`go.mod` du backend utilise des `replace` locaux (à remplacer par des tags avant merge).

```bash
cd ~/ESSENSYS/essensys-server-backend
GOOS=linux GOARCH=arm64 go build -o essensys-server-arm64 ./cmd/server
scp essensys-server-arm64 pi@cm5.local:/opt/essensys/bin/server
```
(Alternative : `go build` natif sur la CM5, avec les 3 repos clonés en siblings.)

### Frontend
```bash
cd ~/ESSENSYS/essensys-server-frontend
bash scripts/sync-plugin-renderer.sh   # resynchronise le renderer partagé
npm run build                           # -> dist/
# servir dist/ via le nginx/traefik existant, ou: npm run preview
```

## 2. Lancer le collecteur sur la CM5

```bash
cd ~/ESSENSYS/essensys-plugin-sungrow
export WINET_IP=192.168.1.247
export WINET_USER=... WINET_PASS=...   # depuis SOPS
export MQTT_HOST=127.0.0.1 MACHINE_ID=A254
python3 collector/sungrow_collector.py --interval 10
```
Vérifier hors-ligne d'abord : `python3 collector/sungrow_collector.py --once --dry-run`.

## 3. Lancer le backend

S'assurer que la config active **MQTT**, **Redis** et **LAN IAM**. Au démarrage,
le log doit afficher : `Framework de plugins activé (/api/plugins/*)`.

## 4. Vérifications (dans l'ordre du flux)

```bash
# 4.1 le collecteur publie
mosquitto_sub -t 'essensys/plugins/sungrow-solar/#' -v      # doit défiler

# 4.2 le backend expose l'API (authentifié : cookie session LAN)
curl -s --cookie "essensys_lan_session=<ID>" \
  http://cm5.local:7070/api/plugins/sungrow-solar/current | jq
# -> { "plugin_id":"sungrow-solar", "samples":[...], "stale":false }

curl -s --cookie "essensys_lan_session=<ID>" \
  http://cm5.local:7070/api/plugins/sungrow-solar/descriptor | jq .title   # "Solaire"

# 4.3 séries dans Prometheus
curl -s http://cm5.local:7070/metrics | grep essensys_plugin_metric

# 4.4 frontend : la tuile "Solaire" apparaît sous le panneau Armoire,
#     avec production PV, injection, batterie. Se connecter en lan_user.
```

### Contrôles de conformité
- **RBAC** : un `lan_guest` doit recevoir **403** sur `/api/plugins/*` (tuile masquée).
- **Stale** : couper le collecteur > 90 s → `current.stale == true`, badge « obsolète » sur la tuile.
- **No-armoire** : aucune requête de mutation armoire émise par la tuile (lecture seule).

## 5. Rollback

- Arrêter le collecteur.
- Backend : sans plugins configurés, `/api/plugins/*` renvoie 404 (aucune régression sur le reste).
- Les séries Prometheus déjà écrites sont conservées.

## Avant merge (hors périmètre test)
- **Taguer** `essensys-plugin-framework/go` et `essensys-plugin-sungrow/adapter` (ex. `v1.0.0`)
  et remplacer les `replace` du `go.mod` backend + adapter par ces versions.
- Adapter le `Dockerfile` (copier les modules taggés / vendoring) — le build image
  actuel ne voit pas les siblings.
- Publier `@essensys/plugin-renderer` sur le registre npm interne (remplace le vendoring).
- Déploiement Ansible du collecteur (tâche 5.5) + ACL Mosquitto par plugin.
