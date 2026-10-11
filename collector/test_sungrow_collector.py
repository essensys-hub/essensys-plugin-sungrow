"""Tests du collecteur Sungrow : configuration WiNet-S (essensys-plugin-sungrow#1).

Lancer : python3 -m unittest discover -s collector -v
"""
import contextlib
import io
import os
import sys
import unittest
from unittest import mock

sys.path.insert(0, os.path.dirname(os.path.abspath(__file__)))
import sungrow_collector as collector  # noqa: E402

# Plage de documentation RFC 5737 : jamais une adresse réelle.
EXAMPLE_IP = "192.0.2.10"
CREDENTIALS = {"WINET_USER": "u", "WINET_PASS": "p"}


def run_main(env, argv):
    """Lance main() avec un environnement WINET_* maîtrisé, sans réseau ni MQTT."""
    base = {k: v for k, v in os.environ.items() if not k.startswith("WINET_")}
    base.update(env)
    stderr = io.StringIO()
    with mock.patch.dict(os.environ, base, clear=True), \
            mock.patch.object(collector, "Publisher") as publisher, \
            mock.patch.object(collector, "read_metrics", return_value={}) as read_metrics, \
            contextlib.redirect_stderr(stderr), \
            contextlib.redirect_stdout(io.StringIO()):
        code = collector.main(argv)
    return code, stderr.getvalue(), publisher, read_metrics


class WinetIpConfigTest(unittest.TestCase):
    def test_refuse_de_demarrer_sans_winet_ip(self):
        for label, env in (("absente", {}), ("vide", {"WINET_IP": ""}), ("blanche", {"WINET_IP": "  "})):
            with self.subTest(winet_ip=label):
                code, err, publisher, read_metrics = run_main({**CREDENTIALS, **env}, ["--once"])
                self.assertEqual(code, 2)
                self.assertIn("WINET_IP", err)
                publisher.assert_not_called()
                read_metrics.assert_not_called()

    def test_interroge_le_dongle_a_l_adresse_fournie(self):
        code, _, publisher, read_metrics = run_main({**CREDENTIALS, "WINET_IP": EXAMPLE_IP}, ["--once"])
        self.assertEqual(code, 0)
        publisher.assert_called_once()
        read_metrics.assert_called_once_with(EXAMPLE_IP, "u", "p")

    def test_dry_run_ne_demande_pas_winet_ip(self):
        code, _, _, read_metrics = run_main({}, ["--once", "--dry-run"])
        self.assertEqual(code, 0)
        read_metrics.assert_not_called()


if __name__ == "__main__":
    unittest.main()
