import os
import sys
import tempfile
import pathlib
import importlib
import unittest


# Ensure env vars are set BEFORE importing the app module (it reads env at import time).
_DATA_DIR = tempfile.mkdtemp(prefix="nixfleet-test-")
os.environ["NIXFLEET_DATA_DIR"] = _DATA_DIR
os.environ["NIXFLEET_DEV_MODE"] = "true"
os.environ["NIXFLEET_PASSWORD_HASH"] = "$2b$12$abcdefghijklmnopqrstuuabcdefghijklmnopqrstuuabcdefghijklmnopqrstuu"
os.environ["NIXFLEET_REQUIRE_TOTP"] = "false"
os.environ["NIXFLEET_SESSION_SECRETS"] = "test-session-secret-1,test-session-secret-2"
os.environ["NIXFLEET_ALLOW_LEGACY_UNSIGNED_SESSION_COOKIE"] = "false"
os.environ["NIXFLEET_API_TOKEN"] = "test-shared-agent-token"
os.environ["NIXFLEET_AGENT_TOKEN_HASH_SECRET"] = "test-agent-hash-secret"
os.environ["NIXFLEET_ALLOW_SHARED_AGENT_TOKEN"] = "true"
os.environ["NIXFLEET_AUTO_PROVISION_AGENT_TOKENS"] = "true"


APP_DIR = pathlib.Path(__file__).resolve().parents[1] / "app"
sys.path.insert(0, str(APP_DIR))
import main as nixfleet_main  # noqa: E402

importlib.reload(nixfleet_main)  # noqa: E402

from fastapi.testclient import TestClient  # noqa: E402


class SecurityTests(unittest.TestCase):
    def test_csp_header_has_nonce_and_no_unsafe_inline(self):
        with TestClient(nixfleet_main.app) as client:
            resp = client.get("/login")
            self.assertEqual(resp.status_code, 200)
            csp = resp.headers.get("content-security-policy", "")
            self.assertIn("script-src 'self' 'nonce-", csp)
            self.assertIn("style-src 'self' 'nonce-", csp)
            self.assertNotIn("'unsafe-inline'", csp)
            # Template should embed the nonce into the <style> tag
            self.assertIn('nonce="', resp.text)

    def test_invalid_signed_session_cookie_is_rejected(self):
        with TestClient(nixfleet_main.app) as client:
            client.cookies.set(nixfleet_main.SESSION_COOKIE_NAME, "v1.bad.bad")
            resp = client.get("/", allow_redirects=False)
            self.assertEqual(resp.status_code, 302)
            self.assertEqual(resp.headers.get("location"), "/login")

    def test_manual_add_host_requires_csrf_and_valid_host_id(self):
        with TestClient(nixfleet_main.app) as client:
            session_token, csrf_token, expires_at = nixfleet_main.create_session()
            client.cookies.set(
                nixfleet_main.SESSION_COOKIE_NAME,
                nixfleet_main.sign_session_cookie(session_token, expires_at),
            )

            # Missing CSRF header -> forbidden
            resp = client.post("/api/hosts", json={"hostname": "testhost"})
            self.assertEqual(resp.status_code, 403)

            # Invalid host ID (underscore disallowed by canonical rules) -> 400
            resp = client.post(
                "/api/hosts",
                json={"hostname": "bad_host"},
                headers={"X-CSRF-Token": csrf_token},
            )
            self.assertEqual(resp.status_code, 400)

            # Valid
            resp = client.post(
                "/api/hosts",
                json={"hostname": "testhost", "host_type": "nixos", "location": "home", "device_type": "server", "theme_color": "#769ff0"},
                headers={"X-CSRF-Token": csrf_token},
            )
            self.assertEqual(resp.status_code, 200)
            data = resp.json()
            self.assertEqual(data["status"], "ok")
            self.assertEqual(data["host_id"], "testhost")

    def test_agent_register_provisions_per_host_token_and_accepts_it(self):
        with TestClient(nixfleet_main.app) as client:
            host_id = "agenttest0"

            # Register with shared token (bootstrap)
            reg = client.post(
                f"/api/hosts/{host_id}/register",
                json={
                    "hostname": host_id,
                    "host_type": "nixos",
                    "location": "home",
                    "device_type": "server",
                    "theme_color": "#769ff0",
                    "criticality": "low",
                    "current_generation": "abc1234",
                },
                headers={"Authorization": f"Bearer {os.environ['NIXFLEET_API_TOKEN']}"},
            )
            self.assertEqual(reg.status_code, 200)
            reg_json = reg.json()
            self.assertEqual(reg_json["status"], "registered")
            self.assertEqual(reg_json["host_id"], host_id)
            self.assertIn("agent_token", reg_json)
            per_host_token = reg_json["agent_token"]
            self.assertTrue(per_host_token)

            # Poll using per-host token should succeed
            poll = client.get(
                f"/api/hosts/{host_id}/poll",
                headers={"Authorization": f"Bearer {per_host_token}"},
            )
            self.assertEqual(poll.status_code, 200)
            self.assertIn("command", poll.json())


if __name__ == "__main__":
    unittest.main()


