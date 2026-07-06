#!/usr/bin/env python3
"""Collecteur MQTT Sungrow WiNet-S pour le framework de plugins Essensys.

Lit l'API locale WiNet-S (wss://<ip>/ws/home/overview) et publie les métriques
sur Mosquitto selon le contrat collecteur :
    essensys/plugins/sungrow-solar/<machine_id>/<metric>   -> {"value","unit","ts"}
    essensys/plugins/sungrow-solar/<machine_id>/_heartbeat -> {"ts"}

Identifiants WiNet fournis par l'environnement (résolus depuis SOPS par
l'orchestrateur, jamais en clair ici) : WINET_IP, WINET_USER, WINET_PASS.
MQTT : MQTT_HOST, MQTT_PORT.

Modes :
    --once            un cycle puis sortie
    --interval N      boucle toutes les N secondes (défaut 10)
    --dry-run         n'ouvre ni device ni MQTT : émet un échantillon simulé
                      et imprime les topics (pour CI/tests hors-ligne)
"""
import argparse
import json
import os
import ssl
import socket
import struct
import base64
import sys
import time

PLUGIN_ID = "sungrow-solar"
LANG = "en_us"

# I18N (WiNet) -> métrique du manifest.
METRIC_MAP = {
    "I18N_COMMON_TOTAL_DCPOWER": ("pv_power", "kW"),
    "I18N_COMMON_LOAD_TOTAL_ACTIVE_POWER": ("load_power", "kW"),
    "I18N_COMMON_FEED_NETWORK_TOTAL_ACTIVE_POWER": ("grid_export_power", "kW"),
    "I18N_CONFIG_KEY_4060": ("grid_import_power", "kW"),
    "I18N_COMMON_BATTERY_SOC": ("battery_soc", "%"),
    "I18N_COMMON_BATTARY_HEALTH": ("battery_soh", "%"),
    "I18N_COMMON_BATTERY_TEMPERATURE": ("battery_temp", "°C"),
    "I18N_COMMON_PV_DAYILY_ENERGY_GENERATION": ("pv_energy_today", "kWh"),
}

SIMULATED = {  # valeurs réelles relevées sur SH6.0RS, pour --dry-run
    "pv_power": 5.81, "load_power": 2.36, "grid_export_power": 3.33,
    "grid_import_power": 0.0, "battery_soc": 100.0, "battery_soh": 99.0,
    "battery_temp": 29.4, "pv_energy_today": 24.8,
}


# ---------- WebSocket WiNet (stdlib) ----------
def ws_connect(ip, timeout=8):
    raw = socket.create_connection((ip, 443), timeout=timeout)
    s = ssl._create_unverified_context().wrap_socket(raw, server_hostname=ip)
    s.settimeout(timeout)
    key = base64.b64encode(os.urandom(16)).decode()
    s.sendall((f"GET /ws/home/overview HTTP/1.1\r\nHost: {ip}\r\nUpgrade: websocket\r\n"
               f"Connection: Upgrade\r\nSec-WebSocket-Key: {key}\r\n"
               f"Sec-WebSocket-Version: 13\r\nOrigin: https://{ip}\r\n\r\n").encode())
    resp = b""
    while b"\r\n\r\n" not in resp:
        resp += s.recv(1)
    if b"101" not in resp.split(b"\r\n")[0]:
        raise RuntimeError("handshake WebSocket échoué")
    return s


def ws_send(s, obj):
    data = json.dumps(obj).encode()
    n, mask = len(data), os.urandom(4)
    hdr = bytearray([0x81])
    if n < 126:
        hdr.append(0x80 | n)
    elif n < 65536:
        hdr.append(0x80 | 126); hdr += struct.pack(">H", n)
    else:
        hdr.append(0x80 | 127); hdr += struct.pack(">Q", n)
    hdr += mask
    s.sendall(bytes(hdr) + bytes(b ^ mask[i % 4] for i, b in enumerate(data)))


def _recvn(s, n):
    buf = b""
    while len(buf) < n:
        c = s.recv(n - len(buf))
        if not c:
            raise ConnectionError("socket fermée")
        buf += c
    return buf


def ws_recv(s):
    b0, b1 = _recvn(s, 2)
    ln = b1 & 0x7F
    if ln == 126:
        ln = struct.unpack(">H", _recvn(s, 2))[0]
    elif ln == 127:
        ln = struct.unpack(">Q", _recvn(s, 8))[0]
    payload = _recvn(s, ln) if ln else b""
    return None if (b0 & 0x0F) == 0x8 else payload.decode(errors="replace")


def recv_service(s, service, max_frames=8):
    for _ in range(max_frames):
        try:
            r = ws_recv(s)
        except socket.timeout:
            return None
        if r is None:
            return None
        try:
            j = json.loads(r)
        except Exception:
            continue
        rd = j.get("result_data", {})
        if isinstance(rd, dict) and (rd.get("service") == service or "token" in rd or "list" in rd):
            return j
    return None


def read_metrics(ip, user, passwd):
    """Retourne {metric: (value, unit)} depuis l'onduleur."""
    s = ws_connect(ip)
    tok = ""

    def call(service, **kw):
        m = {"lang": LANG, "token": tok, "service": service}
        m.update(kw)
        ws_send(s, m)
        return recv_service(s, service)

    tok = call("connect")["result_data"]["token"]
    r = call("login", username=user, passwd=passwd)
    if not r or r.get("result_code") != 1:
        raise RuntimeError("login WiNet refusé")
    tok = r["result_data"].get("token", tok)

    out = {}
    for svc, dev in [("real", 1), ("real_battery", 1)]:
        rr = call(svc, dev_id=str(dev))
        for it in (rr or {}).get("result_data", {}).get("list", []):
            key = it.get("data_name")
            if key in METRIC_MAP:
                name, unit = METRIC_MAP[key]
                try:
                    out[name] = (float(it.get("data_value")), unit)
                except (TypeError, ValueError):
                    pass
    s.close()
    return out


# ---------- Publication MQTT ----------
class Publisher:
    def __init__(self, host, port, dry):
        self.dry = dry
        self.client = None
        if not dry:
            import paho.mqtt.client as mqtt  # dépendance réelle en prod
            self.client = mqtt.Client()
            self.client.connect(host, port, 60)
            self.client.loop_start()

    def publish(self, topic, payload):
        body = json.dumps(payload)
        if self.dry:
            print(f"{topic}  {body}")
        else:
            self.client.publish(topic, body, qos=0, retain=True)


def simulated_metrics():
    unit_by_name = {name: unit for name, unit in METRIC_MAP.values()}
    return {n: (val, unit_by_name.get(n, "")) for n, val in SIMULATED.items()}


def cycle(pub, machine_id, ip, user, passwd, dry):
    metrics = simulated_metrics() if dry else read_metrics(ip, user, passwd)
    ts = int(time.time())
    for name, (value, unit) in metrics.items():
        topic = f"essensys/plugins/{PLUGIN_ID}/{machine_id}/{name}"
        pub.publish(topic, {"value": value, "unit": unit, "ts": ts})
    pub.publish(f"essensys/plugins/{PLUGIN_ID}/{machine_id}/_heartbeat", {"ts": ts})


def main(argv=None):
    ap = argparse.ArgumentParser()
    ap.add_argument("--interval", type=int, default=10)
    ap.add_argument("--once", action="store_true")
    ap.add_argument("--dry-run", action="store_true")
    ap.add_argument("--machine-id", default=os.environ.get("MACHINE_ID", "A254"))
    a = ap.parse_args(argv)

    ip = os.environ.get("WINET_IP", "192.168.1.247")
    user = os.environ.get("WINET_USER", "")
    passwd = os.environ.get("WINET_PASS", "")
    if not a.dry_run and not (user and passwd):
        print("WINET_USER / WINET_PASS requis (via SOPS)", file=sys.stderr)
        return 2

    pub = Publisher(os.environ.get("MQTT_HOST", "127.0.0.1"),
                    int(os.environ.get("MQTT_PORT", "1883")), a.dry_run)

    while True:
        try:
            cycle(pub, a.machine_id, ip, user, passwd, a.dry_run)
        except Exception as e:  # résilience : un cycle raté n'arrête pas le collecteur
            print(f"# cycle erreur: {e}", file=sys.stderr)
        if a.once:
            break
        time.sleep(a.interval)
    return 0


if __name__ == "__main__":
    sys.exit(main())
