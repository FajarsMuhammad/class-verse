#!/usr/bin/env python3
"""Check Make storage defaults using fake credentials; no services required."""
import json
import os
from pathlib import Path
import subprocess
import tempfile
import unittest

MAKEFILE = Path(__file__).resolve().parents[1] / "Makefile"
KEYS = ("STORAGE_ENDPOINT", "STORAGE_BUCKET", "STORAGE_ACCESS_KEY", "STORAGE_SECRET_KEY", "STORAGE_SECURE")


class StorageMakeTest(unittest.TestCase):
    def run_make(self, target, configured=None, provisioned=True):
        with tempfile.TemporaryDirectory() as tmp:
            root = Path(tmp)
            backend = root / "backend"
            backend.mkdir()
            if provisioned:
                (root / ".config").mkdir()
                (root / ".config/storage-app.env").write_text(
                    "TEST_STORAGE_ACCESS_KEY=fake-local-key\nTEST_STORAGE_SECRET_KEY=fake-local-secret\n"
                )
            (backend / ".env").write_text(configured or "")
            go = root / "go"
            go.write_text(
                "#!/usr/bin/env python3\nimport json, os\n"
                "print(json.dumps({k: v for k, v in os.environ.items() if k.startswith('STORAGE_')}))\n"
            )
            go.chmod(0o700)
            env = {k: v for k, v in os.environ.items() if not k.startswith(("STORAGE_", "TEST_STORAGE_", "MAKE"))}
            env["PATH"] = str(root) + os.pathsep + env["PATH"]
            result = subprocess.run(
                ["make", "--silent", "-f", str(MAKEFILE), target, "org=test-org"],
                cwd=backend, env=env, capture_output=True, text=True, check=True,
            )
            return json.loads(result.stdout)

    def test_run_and_seed_use_provisioned_credentials(self):
        for target in ("run", "seed-library"):
            with self.subTest(target=target):
                self.assertEqual(self.run_make(target), dict(zip(KEYS, (
                    "127.0.0.1:9100", "materials", "fake-local-key", "fake-local-secret", "false",
                ))))

    def test_explicit_config_is_not_overwritten(self):
        config = dict(zip(KEYS, ("remote:443", "custom", "fake-explicit-key", "fake-explicit-secret", "true")))
        self.assertEqual(self.run_make("seed-library", "".join(f"{k}={v}\n" for k, v in config.items())), config)

    def test_partial_config_is_not_mixed_with_local_credentials(self):
        self.assertEqual(self.run_make("run", "STORAGE_ENDPOINT=remote:443\n"), {"STORAGE_ENDPOINT": "remote:443"})

    def test_unprovisioned_storage_stays_optional(self):
        self.assertEqual(self.run_make("run", provisioned=False), {})


if __name__ == "__main__":
    unittest.main()
