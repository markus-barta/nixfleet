"""
NixFleet - Fleet management dashboard for NixOS and macOS hosts.

A lightweight solution that supports both NixOS and macOS,
runs in Docker, and provides a web UI for managing host updates.
"""

import os
import sqlite3
import secrets
import hmac
import logging
import re
import asyncio
import json
import ipaddress
import base64
import hashlib
from datetime import datetime, timedelta
from pathlib import Path
from contextlib import contextmanager
from typing import Optional, Set

from fastapi import FastAPI, HTTPException, Request, Depends, Form
from fastapi.responses import HTMLResponse, RedirectResponse, PlainTextResponse, StreamingResponse
from fastapi.staticfiles import StaticFiles
from fastapi.security import HTTPBearer, HTTPAuthorizationCredentials
from pydantic import BaseModel, Field, field_validator
from jinja2 import Environment, FileSystemLoader, select_autoescape
from slowapi import Limiter, _rate_limit_exceeded_handler
from slowapi.util import get_remote_address
from slowapi.errors import RateLimitExceeded

# Optional bcrypt support (falls back to SHA-256 if not available)
try:
    import bcrypt
    BCRYPT_AVAILABLE = True
except ImportError:
    BCRYPT_AVAILABLE = False

# Optional TOTP support
try:
    import pyotp
    TOTP_AVAILABLE = True
except ImportError:
    TOTP_AVAILABLE = False

# ============================================================================
# Configuration
# ============================================================================

DATA_DIR = Path(os.environ.get("NIXFLEET_DATA_DIR", "/data"))
DB_PATH = DATA_DIR / "nixfleet.db"
TEMPLATES_DIR = Path(__file__).parent / "templates"

# Authentication (all required in production)
PASSWORD_HASH = os.environ.get("NIXFLEET_PASSWORD_HASH", "")
TOTP_SECRET = os.environ.get("NIXFLEET_TOTP_SECRET", "")
API_TOKEN = os.environ.get("NIXFLEET_API_TOKEN", "")

# Agent authentication hardening / migration to per-host credentials
AGENT_TOKEN_HASH_SECRET = os.environ.get("NIXFLEET_AGENT_TOKEN_HASH_SECRET", "")
ALLOW_SHARED_AGENT_TOKEN = os.environ.get("NIXFLEET_ALLOW_SHARED_AGENT_TOKEN", "true").lower() in ("1", "true", "yes")
AUTO_PROVISION_AGENT_TOKENS = os.environ.get("NIXFLEET_AUTO_PROVISION_AGENT_TOKENS", "true").lower() in ("1", "true", "yes")

# Security modes
DEV_MODE = os.environ.get("NIXFLEET_DEV_MODE", "").lower() in ("1", "true", "yes")
REQUIRE_TOTP = os.environ.get("NIXFLEET_REQUIRE_TOTP", "").lower() in ("1", "true", "yes")

SESSION_DURATION = timedelta(hours=24)

# Read version from version.json (created at Docker build time)
def _get_version() -> str:
    try:
        version_file = Path(__file__).parent / "version.json"
        if version_file.exists():
            data = json.loads(version_file.read_text())
            return data.get("version", "dev")
    except Exception:
        pass
    return "dev"

VERSION = _get_version()

# Signed session cookies (defense in depth on top of DB sessions)
SESSION_COOKIE_NAME = "nixfleet_session"
_SESSION_SECRETS_RAW = os.environ.get("NIXFLEET_SESSION_SECRETS", "")
SESSION_SECRETS: list[str] = [s.strip() for s in _SESSION_SECRETS_RAW.split(",") if s.strip()]
ALLOW_LEGACY_UNSIGNED_SESSION_COOKIE = os.environ.get(
    "NIXFLEET_ALLOW_LEGACY_UNSIGNED_SESSION_COOKIE",
    "true" if DEV_MODE else "false",
).lower() in ("1", "true", "yes")

# Reverse proxy / client IP handling (logging + rate limiting)
TRUST_PROXY_HEADERS = os.environ.get("NIXFLEET_TRUST_PROXY_HEADERS", "").lower() in ("1", "true", "yes")
_TRUSTED_PROXY_IPS_RAW = os.environ.get("NIXFLEET_TRUSTED_PROXY_IPS", "")
TRUSTED_PROXY_IPS: set[str] = set()
for _ip in [p.strip() for p in _TRUSTED_PROXY_IPS_RAW.split(",") if p.strip()]:
    try:
        TRUSTED_PROXY_IPS.add(str(ipaddress.ip_address(_ip)))
    except ValueError:
        # Ignore invalid IPs; don't fail startup due to a typo
        pass


def get_client_ip(request: Request) -> str:
    """
    Best-effort client IP for logging and rate limiting.

    If behind a trusted reverse proxy, use forwarded headers to avoid collapsing to the proxy IP.
    """
    direct_ip = request.client.host if request.client else "unknown"

    if TRUST_PROXY_HEADERS:
        # If a trusted proxy allowlist is configured, only trust forwarded headers from those proxies.
        if TRUSTED_PROXY_IPS and direct_ip not in TRUSTED_PROXY_IPS:
            return direct_ip

        # Prefer X-Forwarded-For (left-most is original client)
        xff = request.headers.get("x-forwarded-for", "")
        if xff:
            first = xff.split(",")[0].strip()
            if first:
                return first

        # Fallback: X-Real-IP (common with nginx/traefik)
        xri = request.headers.get("x-real-ip", "").strip()
        if xri:
            return xri

    return direct_ip

# UI behavior (dashboard)
# How long the UI should keep action buttons "locked" if no completion event arrives.
# This is a safety fallback for stuck/failed commands; completion should normally unlock earlier.
try:
    UI_ACTION_LOCK_MAX_SECONDS = int(os.environ.get("NIXFLEET_UI_ACTION_LOCK_MAX_SECONDS", "86400"))
except ValueError:
    UI_ACTION_LOCK_MAX_SECONDS = 86400

# Build-time git hash (embedded during docker build, no API calls needed)
# This is the "source of truth" - hosts are outdated if they don't match this
def get_build_git_hash() -> Optional[str]:
    """Get the git hash embedded at build time. No API calls needed."""
    # Try environment variable first (set in docker-compose or Dockerfile)
    env_hash = os.getenv("NIXFLEET_GIT_HASH")
    if env_hash:
        return env_hash
    
    # Fallback: try to read from version.json (created during build)
    version_file = Path(__file__).parent / "version.json"
    if version_file.exists():
        try:
            import json
            with open(version_file) as f:
                data = json.load(f)
                return data.get("gitCommit")
        except Exception:
            pass
    
    # Last resort: try git command (works in dev, not in container)
    try:
        import subprocess
        result = subprocess.run(
            ["git", "rev-parse", "HEAD"],
            capture_output=True, text=True, timeout=2,
            cwd=Path(__file__).parent.parent
        )
        if result.returncode == 0:
            return result.stdout.strip()
    except Exception:
        pass
    
    return None


# Cache the build hash (it never changes during runtime)
_BUILD_GIT_HASH: Optional[str] = None


def get_build_hash() -> Optional[str]:
    """Get NixFleet's own build git hash (embedded at Docker build time). Cached."""
    global _BUILD_GIT_HASH
    if _BUILD_GIT_HASH is None:
        _BUILD_GIT_HASH = get_build_git_hash()
        if _BUILD_GIT_HASH:
            logger.info(f"NixFleet build hash: {_BUILD_GIT_HASH[:7]}")
    return _BUILD_GIT_HASH


# ============================================================================
# Cookie Signing Helpers
# ============================================================================


def _b64url_encode(data: bytes) -> str:
    return base64.urlsafe_b64encode(data).decode("ascii").rstrip("=")


def _b64url_decode(data: str) -> bytes:
    pad = "=" * ((4 - (len(data) % 4)) % 4)
    return base64.urlsafe_b64decode(data + pad)


def sign_session_cookie(session_token: str, expires_at: datetime) -> str:
    """
    Create a signed cookie value for the session token.

    Format: v1.<payload_b64>.<sig_b64>
    """
    if not SESSION_SECRETS:
        # In DEV_MODE we can operate without signing (legacy), but in production startup enforces secrets.
        return session_token

    payload = {
        "v": 1,
        "t": session_token,
        "exp": int(expires_at.timestamp()),
    }
    payload_json = json.dumps(payload, separators=(",", ":"), sort_keys=True).encode("utf-8")
    payload_b64 = _b64url_encode(payload_json)

    sig = hmac.new(SESSION_SECRETS[0].encode("utf-8"), payload_b64.encode("utf-8"), hashlib.sha256).digest()
    sig_b64 = _b64url_encode(sig)
    return f"v1.{payload_b64}.{sig_b64}"


def unsign_session_cookie(cookie_value: str) -> Optional[str]:
    """Verify and decode a signed session cookie. Returns the session token if valid."""
    if not cookie_value or not cookie_value.startswith("v1."):
        return None
    try:
        _, payload_b64, sig_b64 = cookie_value.split(".", 2)
    except ValueError:
        return None

    if not SESSION_SECRETS:
        return None

    try:
        sig_bytes = _b64url_decode(sig_b64)
    except Exception:
        return None

    verified = False
    for secret in SESSION_SECRETS:
        expected = hmac.new(secret.encode("utf-8"), payload_b64.encode("utf-8"), hashlib.sha256).digest()
        if hmac.compare_digest(sig_bytes, expected):
            verified = True
            break
    if not verified:
        return None

    try:
        payload = json.loads(_b64url_decode(payload_b64).decode("utf-8"))
        if payload.get("v") != 1:
            return None
        token = payload.get("t")
        exp = int(payload.get("exp", 0))
        if not token or exp <= 0:
            return None
        if int(datetime.utcnow().timestamp()) > exp:
            return None
        return str(token)
    except Exception:
        return None


# Cache for nixcfg repo info (source of truth for fleet)
# Fetched from GitHub Pages static file (no API rate limits)
_NIXCFG_CACHE: dict = {"hash": None, "message": None, "fetched_at": None}
NIXCFG_CACHE_TTL = 60  # 1 minute (static file is cheap to fetch)
NIXCFG_VERSION_URL = os.environ.get(
    "NIXFLEET_NIXCFG_VERSION_URL",
    "https://markus-barta.github.io/nixcfg/version.json"
)


def get_nixcfg_info() -> tuple[Optional[str], Optional[str]]:
    """Get latest nixcfg commit hash and message from GitHub Pages. Cached for 1 minute."""
    import urllib.request
    import urllib.error
    
    now = datetime.utcnow()
    
    # Return cached value if still valid
    if _NIXCFG_CACHE["fetched_at"]:
        age = (now - _NIXCFG_CACHE["fetched_at"]).total_seconds()
        if age < NIXCFG_CACHE_TTL:
            return _NIXCFG_CACHE["hash"], _NIXCFG_CACHE["message"]
    
    # Fetch from GitHub Pages static file
    try:
        req = urllib.request.Request(NIXCFG_VERSION_URL, headers={"Cache-Control": "no-cache"})
        with urllib.request.urlopen(req, timeout=5) as response:
            data = json.loads(response.read().decode())
            commit_hash = data.get("gitCommit", "") or data.get("hash", "")
            commit_msg = data.get("message", "") or data.get("commitMessage", "")
            
            _NIXCFG_CACHE["hash"] = commit_hash
            _NIXCFG_CACHE["message"] = commit_msg[:80] if commit_msg else ""
            _NIXCFG_CACHE["fetched_at"] = now
            
            if commit_hash:
                logger.info(f"Fetched nixcfg version: {commit_hash[:7]} - {commit_msg[:40]}...")
            return commit_hash, _NIXCFG_CACHE["message"]
    except (urllib.error.URLError, json.JSONDecodeError, KeyError) as e:
        logger.debug(f"Could not fetch nixcfg version (may not be set up yet): {e}")
        # Return stale cache if available
        if _NIXCFG_CACHE["hash"]:
            return _NIXCFG_CACHE["hash"], _NIXCFG_CACHE["message"]
        return None, None


# Backwards compatibility alias
def get_latest_hash() -> Optional[str]:
    """Get the latest nixcfg hash (source of truth for fleet)."""
    hash_val, _ = get_nixcfg_info()
    return hash_val


# Rate limiting (proxy-aware if configured)
limiter = Limiter(key_func=get_client_ip)

# ============================================================================
# SSE (Server-Sent Events) Infrastructure
# ============================================================================

# Connected SSE clients (each is an asyncio.Queue)
sse_clients: Set[asyncio.Queue] = set()
sse_clients_lock = asyncio.Lock()


async def broadcast_event(event_type: str, data: dict):
    """Broadcast an event to all connected SSE clients."""
    async with sse_clients_lock:
        if not sse_clients:
            return
        # Snapshot to avoid iterating a shared set that may be mutated by connect/disconnect
        queues = list(sse_clients)

    event_data = json.dumps({"type": event_type, **data})
    message = f"event: {event_type}\ndata: {event_data}\n\n"
    
    # Send to all clients.
    # If a client is slow and its queue is full, drop this event (do NOT disconnect).
    for queue in queues:
        try:
            queue.put_nowait(message)
        except asyncio.QueueFull:
            metric_inc("sse_queue_drops", 1)
            continue

# Host ID validation pattern (alphanumeric + hyphen, like hostnames)
HOST_ID_PATTERN = re.compile(r"^[a-zA-Z][a-zA-Z0-9\-]{0,62}$")

# ============================================================================
# Logging Setup
# ============================================================================

logging.basicConfig(
    level=logging.INFO,
    format="%(asctime)s [%(levelname)s] %(name)s: %(message)s",
    datefmt="%Y-%m-%d %H:%M:%S",
)
logger = logging.getLogger("nixfleet")

# Simple in-process metrics (best-effort; resets on restart)
METRICS: dict[str, int] = {
    "rate_limit_exceeded": 0,
    "csrf_failures": 0,
    "agent_auth_failures": 0,
    "invalid_signed_session_cookie": 0,
    "sse_queue_drops": 0,
}


def metric_inc(key: str, amount: int = 1):
    try:
        METRICS[key] = METRICS.get(key, 0) + amount
    except Exception:
        pass

# ============================================================================
# Jinja2 Templates
# ============================================================================

jinja_env = Environment(
    loader=FileSystemLoader(str(TEMPLATES_DIR)),
    autoescape=select_autoescape(["html", "xml"]),
)


def render_template(name: str, **context) -> str:
    """Render a Jinja2 template with context."""
    template = jinja_env.get_template(name)
    return template.render(**context)


# ============================================================================
# Database
# ============================================================================


def init_db():
    """Initialize the SQLite database with all tables."""
    DATA_DIR.mkdir(parents=True, exist_ok=True)
    logger.info(f"Initializing database at {DB_PATH}")
    
    with get_db() as conn:
        # Hosts table
        conn.execute("""
            CREATE TABLE IF NOT EXISTS hosts (
                id TEXT PRIMARY KEY,
                hostname TEXT NOT NULL,
                host_type TEXT NOT NULL CHECK(host_type IN ('nixos', 'macos')),
                location TEXT,
                device_type TEXT DEFAULT 'server',
                theme_color TEXT DEFAULT '#769ff0',
                criticality TEXT DEFAULT 'low' CHECK(criticality IN ('low', 'medium', 'high')),
                icon TEXT,
                last_seen TEXT,
                last_switch TEXT,
                last_audit TEXT,
                last_fix TEXT,
                current_generation TEXT,
                status TEXT DEFAULT 'unknown',
                pending_command TEXT,
                command_queued_at TEXT,
                comment TEXT,
                test_status TEXT,
                config_repo TEXT,
                -- Test tracking
                test_running INTEGER DEFAULT 0,
                test_current INTEGER DEFAULT 0,
                test_total INTEGER DEFAULT 0,
                test_passed_count INTEGER DEFAULT 0,
                test_result TEXT,
                poll_interval INTEGER DEFAULT 30,
                -- Metrics (JSON)
                metrics TEXT,
                -- Agent auth (per-host token, stored as a hash)
                agent_token_hash TEXT
            )
        """)
        
        # Command log table
        conn.execute("""
            CREATE TABLE IF NOT EXISTS command_log (
                id INTEGER PRIMARY KEY AUTOINCREMENT,
                host_id TEXT NOT NULL,
                command TEXT NOT NULL,
                status TEXT NOT NULL,
                output TEXT,
                created_at TEXT NOT NULL,
                completed_at TEXT,
                FOREIGN KEY (host_id) REFERENCES hosts(id)
            )
        """)
        
        # Sessions table (persistent across restarts)
        conn.execute("""
            CREATE TABLE IF NOT EXISTS sessions (
                token TEXT PRIMARY KEY,
                csrf_token TEXT NOT NULL,
                expires_at TEXT NOT NULL,
                created_at TEXT NOT NULL
            )
        """)
        
        # Migrations for existing databases
        # (ALTER TABLE fails silently if column exists - we catch the error)
        migrations = [
            "ALTER TABLE hosts ADD COLUMN device_type TEXT DEFAULT 'server'",
            "ALTER TABLE hosts ADD COLUMN theme_color TEXT DEFAULT '#769ff0'",
            "ALTER TABLE hosts ADD COLUMN metrics TEXT",
            "ALTER TABLE hosts ADD COLUMN agent_token_hash TEXT",
        ]
        for migration in migrations:
            try:
                conn.execute(migration)
            except sqlite3.OperationalError:
                pass  # Column already exists
        
        # Create indexes for performance
        conn.execute("CREATE INDEX IF NOT EXISTS idx_hosts_hostname ON hosts(hostname)")
        conn.execute("CREATE INDEX IF NOT EXISTS idx_command_log_host ON command_log(host_id)")
        conn.execute("CREATE INDEX IF NOT EXISTS idx_sessions_expires ON sessions(expires_at)")
        
        conn.commit()
    
    logger.info("Database initialized successfully")


@contextmanager
def get_db():
    """Get a database connection with row factory."""
    conn = sqlite3.connect(str(DB_PATH))
    conn.row_factory = sqlite3.Row
    try:
        yield conn
    finally:
        conn.close()


def cleanup_expired_sessions():
    """Remove expired sessions from database."""
    with get_db() as conn:
        result = conn.execute(
            "DELETE FROM sessions WHERE expires_at < ?",
            (datetime.utcnow().isoformat(),)
        )
        if result.rowcount > 0:
            logger.info(f"Cleaned up {result.rowcount} expired sessions")
        conn.commit()


# ============================================================================
# Session Management (Persistent in SQLite)
# ============================================================================


def create_session() -> tuple[str, str, datetime]:
    """Create a new session token and CSRF token, store in database.
    
    Returns:
        Tuple of (session_token, csrf_token, expires_at)
    """
    token = secrets.token_urlsafe(32)
    csrf_token = secrets.token_urlsafe(32)
    expires_at = datetime.utcnow() + SESSION_DURATION
    
    with get_db() as conn:
        conn.execute(
            "INSERT INTO sessions (token, csrf_token, expires_at, created_at) VALUES (?, ?, ?, ?)",
            (token, csrf_token, expires_at.isoformat(), datetime.utcnow().isoformat())
        )
        conn.commit()
    
    logger.info("New session created")
    return token, csrf_token, expires_at


def get_csrf_token(session_token: str) -> Optional[str]:
    """Get the CSRF token for a session."""
    with get_db() as conn:
        row = conn.execute(
            "SELECT csrf_token FROM sessions WHERE token = ?",
            (session_token,)
        ).fetchone()
        return row["csrf_token"] if row else None


def verify_csrf_token(session_token: str, csrf_token: str) -> bool:
    """Verify CSRF token matches the session."""
    if not session_token or not csrf_token:
        return False
    stored_csrf = get_csrf_token(session_token)
    if not stored_csrf:
        return False
    return hmac.compare_digest(csrf_token, stored_csrf)


def verify_csrf(request: Request):
    """
    Verify CSRF token for session-authenticated API requests.

    Expected to be provided via `X-CSRF-Token` header from the dashboard JS.
    """
    session_token = get_session_token(request)
    csrf_token = request.headers.get("X-CSRF-Token", "")
    if not verify_csrf_token(session_token, csrf_token):
        metric_inc("csrf_failures", 1)
        logger.warning("CSRF validation failed")
        raise HTTPException(status_code=403, detail="Invalid CSRF token")


def verify_session(token: str) -> bool:
    """Verify a session token is valid and not expired."""
    with get_db() as conn:
        row = conn.execute(
            "SELECT expires_at FROM sessions WHERE token = ?",
            (token,)
        ).fetchone()
        
        if not row:
            return False
        
        expires_at = datetime.fromisoformat(row["expires_at"])
        if datetime.utcnow() > expires_at:
            # Clean up expired session
            conn.execute("DELETE FROM sessions WHERE token = ?", (token,))
            conn.commit()
            return False
        
        return True


def delete_session(token: str):
    """Delete a session from database."""
    with get_db() as conn:
        conn.execute("DELETE FROM sessions WHERE token = ?", (token,))
        conn.commit()


# ============================================================================
# Authentication Helpers
# ============================================================================


def hash_password(password: str) -> str:
    """Hash a password using bcrypt."""
    if not BCRYPT_AVAILABLE:
        raise RuntimeError("bcrypt not available - install bcrypt package")
    return bcrypt.hashpw(password.encode(), bcrypt.gensalt()).decode()


def verify_password(password: str) -> bool:
    """Verify password against stored bcrypt hash."""
    if not PASSWORD_HASH:
        logger.warning("No password hash configured!")
        return False
    
    if not BCRYPT_AVAILABLE:
        logger.error("bcrypt not available - cannot verify password!")
        return False
    
    # Validate hash format
    if not (PASSWORD_HASH.startswith("$2b$") or PASSWORD_HASH.startswith("$2a$")):
        logger.error("Invalid password hash format - must be bcrypt ($2b$ or $2a$)")
        return False
    
    try:
        return bcrypt.checkpw(password.encode(), PASSWORD_HASH.encode())
    except Exception as e:
        logger.error(f"bcrypt verification failed: {e}")
        return False


def verify_totp(code: str) -> bool:
    """Verify TOTP code if configured."""
    if not TOTP_SECRET or not TOTP_AVAILABLE:
        return True  # TOTP not configured, skip
    totp = pyotp.TOTP(TOTP_SECRET)
    return totp.verify(code, valid_window=1)


def get_session_token(request: Request) -> Optional[str]:
    """Extract session token from cookie."""
    raw = request.cookies.get(SESSION_COOKIE_NAME)
    if not raw:
        return None
    if raw.startswith("v1."):
        token = unsign_session_cookie(raw)
        if not token:
            metric_inc("invalid_signed_session_cookie", 1)
            logger.warning("Invalid signed session cookie rejected")
        return token
    # Legacy/unsigned cookie support (opt-in, mainly for upgrades)
    if ALLOW_LEGACY_UNSIGNED_SESSION_COOKIE:
        return raw
    logger.warning(
        "Legacy unsigned session cookie rejected (set NIXFLEET_ALLOW_LEGACY_UNSIGNED_SESSION_COOKIE=true to allow)"
    )
    return None


def require_auth(request: Request) -> bool:
    """Check if request is authenticated."""
    token = get_session_token(request)
    if token and verify_session(token):
        return True
    raise HTTPException(status_code=401, detail="Not authenticated")


def verify_api_token(token: str) -> bool:
    """Verify API token for agent requests. Fails closed if token not configured."""
    if not API_TOKEN:
        # SECURITY: Fail closed - no token means no access
        logger.error("Agent auth attempted but NIXFLEET_API_TOKEN not configured")
        return False
    if not token:
        return False
    return hmac.compare_digest(token, API_TOKEN)


def hash_agent_token(token: str) -> str:
    """Hash an agent token for storage in the DB (server-side secret required)."""
    if not AGENT_TOKEN_HASH_SECRET:
        raise RuntimeError("NIXFLEET_AGENT_TOKEN_HASH_SECRET is not configured")
    return hmac.new(
        AGENT_TOKEN_HASH_SECRET.encode("utf-8"),
        token.encode("utf-8"),
        hashlib.sha256,
    ).hexdigest()


def verify_agent_token_hash(stored_hash: str, token: str) -> bool:
    """Verify an agent token against a stored hash."""
    if not stored_hash or not token:
        return False
    try:
        expected = hash_agent_token(token)
    except RuntimeError:
        return False
    return hmac.compare_digest(stored_hash, expected)


def provision_agent_token_for_host(conn: sqlite3.Connection, host_id: str) -> Optional[str]:
    """
    Generate and store a per-host agent token (hashed in DB).
    Returns the raw token (only at provisioning time).
    """
    if not AGENT_TOKEN_HASH_SECRET:
        return None

    token = secrets.token_urlsafe(32)
    token_hash = hash_agent_token(token)
    conn.execute("UPDATE hosts SET agent_token_hash = ? WHERE id = ?", (token_hash, host_id))
    return token


def get_bearer_token(credentials: Optional[HTTPAuthorizationCredentials]) -> str:
    return credentials.credentials if credentials else ""


def verify_agent_auth_for_host(host_id: str, credentials: Optional[HTTPAuthorizationCredentials]) -> dict:
    """
    Verify agent auth for a specific host.

    Supports:
    - Per-host tokens (stored hashed in DB)
    - Shared token (NIXFLEET_API_TOKEN) for migration/compat, if enabled

    Returns a context dict: { "ok": bool, "used_shared": bool }
    """
    token = get_bearer_token(credentials)
    if not token:
        return {"ok": False, "used_shared": False}

    with get_db() as conn:
        row = conn.execute("SELECT agent_token_hash FROM hosts WHERE id = ?", (host_id,)).fetchone()
        stored_hash = row["agent_token_hash"] if row else None

        if stored_hash:
            if verify_agent_token_hash(stored_hash, token):
                return {"ok": True, "used_shared": False}

            if ALLOW_SHARED_AGENT_TOKEN and API_TOKEN and hmac.compare_digest(token, API_TOKEN):
                return {"ok": True, "used_shared": True}

            return {"ok": False, "used_shared": False}

        # No per-host token set yet
        if ALLOW_SHARED_AGENT_TOKEN and API_TOKEN and hmac.compare_digest(token, API_TOKEN):
            return {"ok": True, "used_shared": True}

        return {"ok": False, "used_shared": False}


# ============================================================================
# Pydantic Models with Validation
# ============================================================================

# Regex patterns for validation
HOSTNAME_PATTERN = re.compile(r"^[a-zA-Z][a-zA-Z0-9\-]{0,62}$")
GENERATION_PATTERN = re.compile(r"^[a-zA-Z0-9\-]{1,64}$")


class HostRegistration(BaseModel):
    """Model for host registration requests."""
    hostname: str = Field(..., min_length=1, max_length=63)
    host_type: str = Field(..., pattern="^(nixos|macos)$")
    location: Optional[str] = Field(None, max_length=50)
    device_type: Optional[str] = Field("server", pattern="^(server|desktop|laptop|gaming)$")
    theme_color: Optional[str] = Field("#769ff0", pattern="^#[0-9a-fA-F]{6}$")
    criticality: Optional[str] = Field("low", pattern="^(low|medium|high)$")
    icon: Optional[str] = Field(None, max_length=10)
    current_generation: Optional[str] = Field(None, max_length=64)
    comment: Optional[str] = Field(None, max_length=500)
    test_status: Optional[str] = Field(None, max_length=100)
    config_repo: Optional[str] = Field(None, max_length=200)
    poll_interval: Optional[int] = Field(None, ge=1, le=3600)
    metrics: Optional[dict] = Field(None)  # StaSysMo metrics (cpu, ram, swap, load)

    @field_validator("hostname")
    @classmethod
    def validate_hostname(cls, v: str) -> str:
        if not HOSTNAME_PATTERN.match(v):
            raise ValueError("Invalid hostname format")
        return v.lower()

    @field_validator("current_generation")
    @classmethod
    def validate_generation(cls, v: Optional[str]) -> Optional[str]:
        if v and not GENERATION_PATTERN.match(v):
            raise ValueError("Invalid generation format")
        return v


class HostStatus(BaseModel):
    """Model for host status updates."""
    status: str = Field(..., pattern="^(ok|error|unknown)$")
    current_generation: Optional[str] = Field(None, max_length=64)
    output: Optional[str] = Field(None, max_length=10000)
    test_status: Optional[str] = Field(None, max_length=100)
    comment: Optional[str] = Field(None, max_length=200)

    @field_validator("output")
    @classmethod
    def sanitize_output(cls, v: Optional[str]) -> Optional[str]:
        if v:
            # Remove any control characters except newlines
            return "".join(c for c in v if c == "\n" or (ord(c) >= 32 and ord(c) < 127))
        return v


class HostUpdate(BaseModel):
    """Model for host metadata updates."""
    comment: Optional[str] = Field(None, max_length=500)
    criticality: Optional[str] = Field(None, pattern="^(low|medium|high)$")
    last_audit: Optional[str] = Field(None, max_length=30)
    last_fix: Optional[str] = Field(None, max_length=30)
    test_status: Optional[str] = Field(None, max_length=100)


class CommandRequest(BaseModel):
    """Model for command queue requests."""
    command: str = Field(..., pattern="^(pull|switch|pull-switch|test|stop|restart)$")


# ============================================================================
# FastAPI App
# ============================================================================

app = FastAPI(
    title="NixFleet",
    description="Fleet management for NixOS and macOS hosts",
    version=VERSION,
)
app.mount("/static", StaticFiles(directory=str(Path(__file__).parent / "static")), name="static")
app.state.limiter = limiter
security = HTTPBearer(auto_error=False)


@app.exception_handler(RateLimitExceeded)
async def rate_limit_exceeded_handler(request: Request, exc: RateLimitExceeded):
    metric_inc("rate_limit_exceeded", 1)
    logger.warning(f"Rate limit exceeded: ip={get_client_ip(request)} path={request.url.path}")
    return await _rate_limit_exceeded_handler(request, exc)


@app.middleware("http")
async def security_headers_middleware(request: Request, call_next):
    """Add security headers to all responses."""
    # Per-request nonce for CSP (templates embed it into <style>/<script> tags)
    request.state.csp_nonce = secrets.token_urlsafe(18)

    response = await call_next(request)
    
    nonce = getattr(request.state, "csp_nonce", "")

    # Security headers (tight in production; still set CSP in dev to catch regressions)
    if not DEV_MODE:
        response.headers["Strict-Transport-Security"] = "max-age=31536000; includeSubDomains"
        response.headers["X-Content-Type-Options"] = "nosniff"
        response.headers["X-Frame-Options"] = "DENY"
        response.headers["X-XSS-Protection"] = "1; mode=block"
        response.headers["Referrer-Policy"] = "strict-origin-when-cross-origin"

    # No 'unsafe-inline' - templates must use nonce + no inline handlers/styles.
    response.headers["Content-Security-Policy"] = (
        "default-src 'self'; "
        f"script-src 'self' 'nonce-{nonce}'; "
        f"style-src 'self' 'nonce-{nonce}'; "
        "img-src 'self' data:; "
        "connect-src 'self'; "
        "font-src 'self'; "
        "object-src 'none'; "
        "base-uri 'none'; "
        "frame-ancestors 'none'; "
        "form-action 'self'"
    )
    
    return response


def validate_host_id(host_id: str) -> str:
    """Validate and sanitize host_id parameter."""
    if not HOST_ID_PATTERN.match(host_id):
        raise HTTPException(status_code=400, detail="Invalid host ID format")
    return host_id.lower()


@app.on_event("startup")
async def startup():
    """Initialize database, validate config, and cleanup expired sessions."""
    # SECURITY: Validate required configuration
    errors = []
    
    if not BCRYPT_AVAILABLE:
        errors.append("bcrypt package not installed - required for password hashing")
    
    if not PASSWORD_HASH:
        errors.append("NIXFLEET_PASSWORD_HASH not set")
    elif not (PASSWORD_HASH.startswith("$2b$") or PASSWORD_HASH.startswith("$2a$")):
        errors.append("NIXFLEET_PASSWORD_HASH must be a bcrypt hash (starts with $2b$ or $2a$)")
    
    if ALLOW_SHARED_AGENT_TOKEN and not API_TOKEN and not DEV_MODE:
        errors.append("NIXFLEET_API_TOKEN not set (required in production when NIXFLEET_ALLOW_SHARED_AGENT_TOKEN=true)")

    if (AUTO_PROVISION_AGENT_TOKENS or not ALLOW_SHARED_AGENT_TOKEN) and not AGENT_TOKEN_HASH_SECRET and not DEV_MODE:
        errors.append("NIXFLEET_AGENT_TOKEN_HASH_SECRET not set (required for per-host agent tokens)")

    if not SESSION_SECRETS and not DEV_MODE:
        errors.append("NIXFLEET_SESSION_SECRETS not set (required in production for signed session cookies)")
    
    if REQUIRE_TOTP and not TOTP_SECRET:
        errors.append("NIXFLEET_REQUIRE_TOTP is set but NIXFLEET_TOTP_SECRET is missing")
    
    if REQUIRE_TOTP and not TOTP_AVAILABLE:
        errors.append("NIXFLEET_REQUIRE_TOTP is set but pyotp is not installed")
    
    if errors:
        for err in errors:
            logger.error(f"Configuration error: {err}")
        raise RuntimeError(f"NixFleet startup failed: {', '.join(errors)}")
    
    init_db()
    cleanup_expired_sessions()
    logger.info(f"NixFleet v{VERSION} started")
    
    # Warnings for non-fatal issues
    if DEV_MODE:
        logger.warning("DEV_MODE enabled - security features relaxed for development")
    if not API_TOKEN and ALLOW_SHARED_AGENT_TOKEN:
        logger.warning("NIXFLEET_API_TOKEN not set - agents cannot connect (shared token mode)")
    if API_TOKEN and ALLOW_SHARED_AGENT_TOKEN:
        logger.warning("Shared agent token mode enabled (NIXFLEET_ALLOW_SHARED_AGENT_TOKEN=true) - consider migrating to per-host tokens")
    if not TOTP_SECRET:
        logger.info("TOTP not configured - 2FA disabled")


# ============================================================================
# Health Endpoint
# ============================================================================


@app.get("/health")
async def health_check():
    """
    Health check endpoint for monitoring.
    
    SECURITY: Does not expose configuration details (TOTP, API token status).
    Only returns operational health and sanitized metrics.
    """
    try:
        with get_db() as conn:
            conn.execute("SELECT 1").fetchone()
        
        # Only return minimal operational info - no security config leakage
        return {
            "status": "healthy",
            "version": VERSION,
        }
    except Exception as e:
        logger.error(f"Health check failed: {e}")
        raise HTTPException(status_code=503, detail="Service unhealthy")


@app.get("/api/metrics")
async def metrics_endpoint(request: Request):
    """Authenticated metrics snapshot (best-effort, resets on restart)."""
    require_auth(request)
    return {
        "version": VERSION,
        "build_hash": (get_build_hash() or "")[:7] or None,
        "metrics": dict(METRICS),
    }


@app.post("/api/nixcfg/refresh")
async def refresh_nixcfg_target(request: Request):
    """Force refresh the nixcfg target hash (clears cache and refetches)."""
    require_auth(request)
    verify_csrf(request)
    
    # Clear the cache to force a refetch
    global _NIXCFG_CACHE
    _NIXCFG_CACHE["fetched_at"] = None
    
    # Fetch fresh data
    hash_val, message = get_nixcfg_info()
    
    logger.info(f"Refreshed nixcfg target: {hash_val[:7] if hash_val else 'None'}")
    
    return {
        "hash": hash_val[:7] if hash_val else None,
        "full_hash": hash_val,
        "message": message or "",
    }


# ============================================================================
# SSE Endpoint
# ============================================================================


@app.get("/api/events")
async def sse_events(request: Request):
    """
    Server-Sent Events endpoint for real-time dashboard updates.
    Requires session authentication.
    """
    # Verify session
    token = get_session_token(request)
    if not token or not verify_session(token):
        raise HTTPException(status_code=401, detail="Not authenticated")
    
    async def event_generator():
        queue: asyncio.Queue = asyncio.Queue(maxsize=100)
        async with sse_clients_lock:
            sse_clients.add(queue)
            total = len(sse_clients)
        logger.info(f"SSE client connected (total: {total})")
        
        try:
            # Send initial connection event
            yield f"event: connected\ndata: {json.dumps({'clients': total})}\n\n"
            
            # Keep connection alive and send events
            while True:
                try:
                    # Wait for events with timeout (keepalive)
                    message = await asyncio.wait_for(queue.get(), timeout=30.0)
                    yield message
                except asyncio.TimeoutError:
                    # Send keepalive ping
                    yield f": keepalive\n\n"
                    
                # Check if client disconnected
                if await request.is_disconnected():
                    break
                    
        except asyncio.CancelledError:
            pass
        finally:
            async with sse_clients_lock:
                sse_clients.discard(queue)
                remaining = len(sse_clients)
            logger.info(f"SSE client disconnected (remaining: {remaining})")
    
    return StreamingResponse(
        event_generator(),
        media_type="text/event-stream",
        headers={
            "Cache-Control": "no-cache",
            "Connection": "keep-alive",
            "X-Accel-Buffering": "no",  # Disable nginx buffering
        }
    )


# ============================================================================
# Authentication Endpoints
# ============================================================================


@app.get("/login", response_class=HTMLResponse)
async def login_page(request: Request, error: str = ""):
    """Show login page."""
    token = get_session_token(request)
    if token and verify_session(token):
        return RedirectResponse(url="/", status_code=302)

    return render_template(
        "login.html",
        error=error,
        totp_enabled=bool(TOTP_SECRET and TOTP_AVAILABLE),
        csp_nonce=getattr(request.state, "csp_nonce", ""),
    )


@app.post("/login")
@limiter.limit("5/minute")  # Max 5 login attempts per minute per IP
async def login(request: Request, password: str = Form(...), totp: str = Form("")):
    """Process login with rate limiting."""
    logger.info(f"Login attempt from {get_client_ip(request)}")
    
    if not verify_password(password):
        logger.warning("Login failed: invalid password")
        return RedirectResponse(url="/login?error=Invalid+password", status_code=302)

    if TOTP_SECRET and TOTP_AVAILABLE and (not totp or not verify_totp(totp)):
        logger.warning("Login failed: invalid TOTP")
        return RedirectResponse(url="/login?error=Invalid+TOTP+code", status_code=302)

    session_token, _, expires_at = create_session()  # CSRF token stored in DB, retrieved per-request
    response = RedirectResponse(url="/", status_code=302)
    response.set_cookie(
        key=SESSION_COOKIE_NAME,
        value=sign_session_cookie(session_token, expires_at),
        httponly=True,
        secure=not DEV_MODE,  # Disable secure in dev mode for localhost
        samesite="strict" if not DEV_MODE else "lax",
        max_age=int(SESSION_DURATION.total_seconds()),
    )
    
    logger.info("Login successful")
    return response


@app.post("/logout")
async def logout(request: Request, csrf_token: str = Form("")):
    """Logout and clear session. POST-only with CSRF protection."""
    session_token = get_session_token(request)
    
    # Validate CSRF token
    if session_token and not verify_csrf_token(session_token, csrf_token):
        logger.warning("Logout CSRF validation failed")
        raise HTTPException(status_code=403, detail="Invalid CSRF token")
    
    if session_token:
        delete_session(session_token)
        logger.info("User logged out")

    response = RedirectResponse(url="/login", status_code=302)
    response.delete_cookie(SESSION_COOKIE_NAME)
    return response


# ============================================================================
# API Endpoints
# ============================================================================


def verify_agent_auth(credentials: Optional[HTTPAuthorizationCredentials] = Depends(security)):
    """Verify agent API token. Always required - fails closed."""
    if not credentials or not verify_api_token(credentials.credentials):
        logger.warning("Agent auth failed - missing or invalid token")
        raise HTTPException(status_code=401, detail="Invalid API token")
    return True


@app.get("/api/hosts")
async def list_hosts(request: Request):
    """List all registered hosts."""
    require_auth(request)
    with get_db() as conn:
        rows = conn.execute("SELECT * FROM hosts ORDER BY hostname").fetchall()
        hosts = []
        for row in rows:
            host = dict(row)
            if host["last_seen"]:
                last_seen = datetime.fromisoformat(host["last_seen"])
                host["online"] = datetime.utcnow() - last_seen < timedelta(minutes=5)
            else:
                host["online"] = False
            hosts.append(host)
        return {"hosts": hosts}


class ManualHostCreate(BaseModel):
    """Request body for manually adding a host."""
    hostname: str
    host_type: str = "nixos"
    location: str = "home"
    device_type: str = "server"
    theme_color: str = "#769ff0"


@app.post("/api/hosts")
async def add_host_manually(request: Request, host_data: ManualHostCreate):
    """Manually add a host (for offline/planned hosts)."""
    require_auth(request)
    verify_csrf(request)
    
    # Canonicalize host_id from hostname and validate using the same rules as the rest of the API.
    hostname = host_data.hostname.strip().lower()
    host_id = validate_host_id(hostname)
    
    with get_db() as conn:
        # Check if host already exists
        existing = conn.execute("SELECT id FROM hosts WHERE id = ?", (host_id,)).fetchone()
        if existing:
            raise HTTPException(status_code=409, detail=f"Host '{hostname}' already exists")
        
        # Insert new host (offline, no last_seen)
        conn.execute("""
            INSERT INTO hosts (id, hostname, host_type, location, device_type, theme_color,
                criticality, icon, current_generation, last_seen, status, comment,
                config_repo, poll_interval, metrics)
            VALUES (?, ?, ?, ?, ?, ?, ?, ?, ?, NULL, 'unknown', ?, ?, ?, ?)
        """, (
            host_id, hostname, host_data.host_type, host_data.location,
            host_data.device_type, host_data.theme_color,
            "low", None, None,
            "Manually added, awaiting agent connection", None, 30, None,
        ))
        conn.commit()
    
    logger.info(f"Host manually added: {host_id} ({hostname})")
    
    # Broadcast SSE event
    await broadcast_event("host_update", {
        "host_id": host_id,
        "hostname": hostname,
        "host_type": host_data.host_type,
        "location": host_data.location,
        "device_type": host_data.device_type,
        "theme_color": host_data.theme_color,
        "status": "unknown",
        "online": False,
    })
    
    return {"status": "ok", "host_id": host_id, "message": f"Host '{hostname}' added successfully"}


@app.post("/api/hosts/{host_id}/register")
@limiter.limit("30/minute")  # Limit registration attempts
async def register_host(
    request: Request,
    host_id: str,
    registration: HostRegistration,
    credentials: Optional[HTTPAuthorizationCredentials] = Depends(security),
):
    """Register or update a host."""
    host_id = validate_host_id(host_id)
    auth = verify_agent_auth_for_host(host_id, credentials)
    if not auth["ok"]:
        metric_inc("agent_auth_failures", 1)
        logger.warning(f"Agent auth failed for register: host_id={host_id}")
        raise HTTPException(status_code=401, detail="Invalid API token")

    logger.info(f"Host registration: {host_id} ({registration.hostname}, gen={registration.current_generation})")
    
    provisioned_agent_token: Optional[str] = None
    with get_db() as conn:
        existing = conn.execute("SELECT * FROM hosts WHERE id = ?", (host_id,)).fetchone()
        
        # Serialize metrics to JSON if present
        metrics_json = json.dumps(registration.metrics) if registration.metrics else None
        
        if existing:
            # Update existing host - agent is alive, update last_seen
            conn.execute("""
                UPDATE hosts SET
                    hostname = ?, host_type = ?, location = COALESCE(?, location),
                    device_type = COALESCE(?, device_type),
                    theme_color = COALESCE(?, theme_color),
                    current_generation = ?, last_seen = ?, status = 'ok',
                    config_repo = COALESCE(?, config_repo),
                    poll_interval = COALESCE(?, poll_interval),
                    metrics = COALESCE(?, metrics)
                WHERE id = ?
            """, (
                registration.hostname, registration.host_type, registration.location,
                registration.device_type, registration.theme_color,
                registration.current_generation, datetime.utcnow().isoformat(),
                registration.config_repo, registration.poll_interval, metrics_json, host_id,
            ))
        else:
            # New host - do NOT set last_seen (offline until agent actually polls)
            conn.execute("""
                INSERT INTO hosts (id, hostname, host_type, location, device_type, theme_color,
                    criticality, icon, current_generation, last_seen, status, comment, 
                    config_repo, poll_interval, metrics)
                VALUES (?, ?, ?, ?, ?, ?, ?, ?, ?, NULL, 'ok', ?, ?, ?, ?)
            """, (
                host_id, registration.hostname, registration.host_type, registration.location,
                registration.device_type or "server", registration.theme_color or "#769ff0",
                registration.criticality or "low", registration.icon,
                registration.current_generation,
                registration.comment, registration.config_repo, registration.poll_interval or 30,
                metrics_json,
            ))

        # Auto-provision per-host agent token for migration/hardening.
        if AUTO_PROVISION_AGENT_TOKENS and AGENT_TOKEN_HASH_SECRET:
            row = conn.execute("SELECT agent_token_hash FROM hosts WHERE id = ?", (host_id,)).fetchone()
            if row and not row["agent_token_hash"]:
                provisioned_agent_token = provision_agent_token_for_host(conn, host_id)

        conn.commit()
    
    # Broadcast SSE event
    await broadcast_event("host_update", {
        "host_id": host_id,
        "hostname": registration.hostname,
        "host_type": registration.host_type,
        "location": registration.location,
        "device_type": registration.device_type,
        "theme_color": registration.theme_color,
        "status": "ok",
        "online": True,
        "current_generation": registration.current_generation,
        "last_seen": datetime.utcnow().isoformat(),
        "metrics": registration.metrics,
    })
    
    resp = {"status": "registered", "host_id": host_id}
    if provisioned_agent_token:
        resp["agent_token"] = provisioned_agent_token
    return resp


@app.patch("/api/hosts/{host_id}")
async def update_host(host_id: str, update: HostUpdate, request: Request):
    """Update host metadata."""
    host_id = validate_host_id(host_id)
    require_auth(request)
    logger.info(f"Updating host metadata: {host_id}")
    
    with get_db() as conn:
        updates = []
        params = []
        
        if update.comment is not None:
            updates.append("comment = ?")
            params.append(update.comment)
        if update.criticality is not None:
            updates.append("criticality = ?")
            params.append(update.criticality)
        if update.last_audit is not None:
            updates.append("last_audit = ?")
            params.append(update.last_audit)
        if update.last_fix is not None:
            updates.append("last_fix = ?")
            params.append(update.last_fix)
        if update.test_status is not None:
            updates.append("test_status = ?")
            params.append(update.test_status)
        
        if updates:
            params.append(host_id)
            conn.execute(f"UPDATE hosts SET {', '.join(updates)} WHERE id = ?", params)
            conn.commit()
    
    return {"status": "updated"}


@app.post("/api/hosts/{host_id}/agent-token")
async def rotate_host_agent_token(host_id: str, request: Request):
    """
    Rotate (regenerate) the per-host agent token.

    Requires session auth + CSRF. Returns the raw token once.
    """
    host_id = validate_host_id(host_id)
    require_auth(request)
    verify_csrf(request)

    if not AGENT_TOKEN_HASH_SECRET:
        raise HTTPException(status_code=500, detail="Agent token hashing is not configured")

    new_token = secrets.token_urlsafe(32)
    new_hash = hash_agent_token(new_token)

    with get_db() as conn:
        existing = conn.execute("SELECT id FROM hosts WHERE id = ?", (host_id,)).fetchone()
        if not existing:
            raise HTTPException(status_code=404, detail="Host not found")
        conn.execute("UPDATE hosts SET agent_token_hash = ? WHERE id = ?", (new_hash, host_id))
        conn.commit()

    logger.info(f"Rotated agent token for host: {host_id}")
    return {"status": "ok", "host_id": host_id, "agent_token": new_token}


@app.get("/api/hosts/{host_id}/poll")
@limiter.limit("60/minute")  # Allow frequent polling
async def poll_commands(
    request: Request,
    host_id: str,
    credentials: Optional[HTTPAuthorizationCredentials] = Depends(security),
):
    """Agent polls for pending commands. Auto-registers host if not exists."""
    host_id = validate_host_id(host_id)
    auth = verify_agent_auth_for_host(host_id, credentials)
    if not auth["ok"]:
        metric_inc("agent_auth_failures", 1)
        logger.warning(f"Agent auth failed for poll: host_id={host_id}")
        raise HTTPException(status_code=401, detail="Invalid API token")

    now = datetime.utcnow().isoformat()
    
    provisioned_agent_token: Optional[str] = None
    with get_db() as conn:
        # Check if host exists
        existing = conn.execute("SELECT id FROM hosts WHERE id = ?", (host_id,)).fetchone()
        
        if existing:
            # Update last_seen for existing host
            conn.execute("UPDATE hosts SET last_seen = ? WHERE id = ?", (now, host_id))
        else:
            # Auto-register new host with minimal info (agent will send full info on next register)
            logger.info(f"Auto-registering new host from poll: {host_id}")
            conn.execute("""
                INSERT INTO hosts (id, hostname, host_type, location, device_type, theme_color,
                    criticality, last_seen, status, poll_interval)
                VALUES (?, ?, 'nixos', 'home', 'server', '#769ff0', 'low', ?, 'ok', 30)
            """, (host_id, host_id, now))

        # Auto-provision per-host agent token for migration/hardening.
        if AUTO_PROVISION_AGENT_TOKENS and AGENT_TOKEN_HASH_SECRET:
            row = conn.execute("SELECT agent_token_hash FROM hosts WHERE id = ?", (host_id,)).fetchone()
            if row and not row["agent_token_hash"]:
                provisioned_agent_token = provision_agent_token_for_host(conn, host_id)

        conn.commit()
        
        row = conn.execute(
            "SELECT pending_command FROM hosts WHERE id = ?",
            (host_id,)
        ).fetchone()
        
        if row and row["pending_command"]:
            command = row["pending_command"]
            logger.info(f"Sending command to {host_id}: {command}")
            
            conn.execute("UPDATE hosts SET pending_command = NULL WHERE id = ?", (host_id,))
            conn.execute("""
                INSERT INTO command_log (host_id, command, status, created_at)
                VALUES (?, ?, 'running', ?)
            """, (host_id, command, datetime.utcnow().isoformat()))
            conn.commit()
            
            resp = {"command": command}
            if provisioned_agent_token:
                resp["agent_token"] = provisioned_agent_token
            return resp
    
    resp = {"command": None}
    if provisioned_agent_token:
        resp["agent_token"] = provisioned_agent_token
    return resp


@app.post("/api/hosts/{host_id}/status")
@limiter.limit("30/minute")  # Limit status updates
async def update_status(
    request: Request,
    host_id: str,
    status: HostStatus,
    credentials: Optional[HTTPAuthorizationCredentials] = Depends(security),
):
    """Agent reports command result."""
    host_id = validate_host_id(host_id)
    auth = verify_agent_auth_for_host(host_id, credentials)
    if not auth["ok"]:
        metric_inc("agent_auth_failures", 1)
        logger.warning(f"Agent auth failed for status: host_id={host_id}")
        raise HTTPException(status_code=401, detail="Invalid API token")

    logger.info(f"Status update from {host_id}: {status.status}")
    
    # Generate comment from output if not provided
    comment = status.comment
    if not comment and status.output:
        # Use first line of output, truncated
        first_line = status.output.split('\n')[0][:100]
        if status.status == 'error':
            comment = first_line
        elif 'Already up to date' in status.output or 'Already up-to-date' in status.output:
            comment = 'Already up to date'
        elif 'Updating' in status.output:
            comment = 'Pull successful'
    
    with get_db() as conn:
        conn.execute("""
            UPDATE hosts SET
                status = ?, current_generation = COALESCE(?, current_generation),
                last_seen = ?, test_status = COALESCE(?, test_status),
                comment = COALESCE(?, comment),
                last_switch = CASE WHEN ? IN ('ok', 'success') THEN ? ELSE last_switch END
            WHERE id = ?
        """, (
            status.status, status.current_generation, datetime.utcnow().isoformat(),
            status.test_status, comment, status.status, datetime.utcnow().isoformat(), host_id,
        ))
        
        conn.execute("""
            UPDATE command_log SET status = ?, output = ?, completed_at = ?
            WHERE host_id = ? AND completed_at IS NULL
            ORDER BY created_at DESC LIMIT 1
        """, (status.status, status.output, datetime.utcnow().isoformat(), host_id))
        conn.commit()
    
    # Check if host is now up-to-date
    latest_hash = get_latest_hash()
    host_gen = status.current_generation
    outdated = False
    if host_gen and latest_hash:
        outdated = not latest_hash.startswith(host_gen[:7]) and not host_gen.startswith(latest_hash[:7])
    
    # Broadcast SSE event
    await broadcast_event("host_update", {
        "host_id": host_id,
        "status": status.status,
        "online": True,
        "current_generation": status.current_generation,
        "test_status": status.test_status,
        "comment": comment,
        "last_seen": datetime.utcnow().isoformat(),
        "pending_command": None,  # Command completed
        "test_running": False,
        "outdated": outdated,
    })
    
    return {"status": "updated"}


class TestProgress(BaseModel):
    """Model for test progress updates from agent."""
    current: int = Field(..., ge=0, description="Current test number")
    total: int = Field(..., ge=0, description="Total number of tests")
    passed: int = Field(0, ge=0, description="Number of passed tests so far")
    running: bool = Field(True, description="Whether tests are still running")
    result: Optional[str] = Field(None, description="Final result summary")
    comment: Optional[str] = Field(None, description="Error message or notes")


@app.post("/api/hosts/{host_id}/test-progress")
@limiter.limit("60/minute")
async def update_test_progress(
    request: Request,
    host_id: str,
    progress: TestProgress,
    credentials: Optional[HTTPAuthorizationCredentials] = Depends(security),
):
    """Agent reports test progress."""
    host_id = validate_host_id(host_id)
    auth = verify_agent_auth_for_host(host_id, credentials)
    if not auth["ok"]:
        metric_inc("agent_auth_failures", 1)
        logger.warning(f"Agent auth failed for test-progress: host_id={host_id}")
        raise HTTPException(status_code=401, detail="Invalid API token")

    logger.info(f"Test progress from {host_id}: {progress.current}/{progress.total} (passed: {progress.passed})")
    
    with get_db() as conn:
        if progress.running:
            # Test in progress
            conn.execute("""
                UPDATE hosts SET
                    test_running = 1,
                    test_current = ?,
                    test_total = ?,
                    test_passed_count = ?,
                    last_seen = ?
                WHERE id = ?
            """, (progress.current, progress.total, progress.passed, datetime.utcnow().isoformat(), host_id))
        else:
            # Test completed
            conn.execute("""
                UPDATE hosts SET
                    test_running = 0,
                    test_current = ?,
                    test_total = ?,
                    test_passed_count = ?,
                    test_result = ?,
                    test_status = ?,
                    comment = COALESCE(?, comment),
                    last_seen = ?,
                    last_audit = ?,
                    pending_command = NULL
                WHERE id = ?
            """, (
                progress.current, progress.total, progress.passed,
                progress.result,
                f"{progress.passed}/{progress.total} passed",
                progress.comment,
                datetime.utcnow().isoformat(),
                datetime.utcnow().isoformat(),  # Tests complete = audit done
                host_id
            ))
        conn.commit()
    
    # Broadcast SSE event for live test progress
    await broadcast_event("test_progress", {
        "host_id": host_id,
        "test_running": progress.running,
        "test_current": progress.current,
        "test_total": progress.total,
        "test_passed_count": progress.passed,
        "test_result": progress.result,
        "online": True,
        "last_seen": datetime.utcnow().isoformat(),
    })
    
    return {"status": "updated"}


@app.post("/api/hosts/{host_id}/command")
async def queue_command(host_id: str, request_body: CommandRequest, request: Request):
    """Queue a command for a host. Requires session auth + CSRF token."""
    host_id = validate_host_id(host_id)
    require_auth(request)
    
    # Validate CSRF token from header
    session_token = get_session_token(request)
    csrf_token = request.headers.get("X-CSRF-Token", "")
    if not verify_csrf_token(session_token, csrf_token):
        logger.warning(f"CSRF validation failed for command to {host_id}")
        raise HTTPException(status_code=403, detail="Invalid CSRF token")
    
    logger.info(f"Queueing command for {host_id}: {request_body.command}")
    
    now = datetime.utcnow().isoformat()
    
    with get_db() as conn:
        if request_body.command == "stop":
            # Stop command: clear pending command AND test state immediately (no queue)
            result = conn.execute(
                """UPDATE hosts SET 
                    pending_command = NULL,
                    test_running = 0,
                    status = 'ok'
                WHERE id = ?""",
                (host_id,)
            )
        elif request_body.command == "test":
            # Test command initializes test state
            result = conn.execute(
                """UPDATE hosts SET 
                    pending_command = 'test',
                    command_queued_at = ?,
                    test_running = 1,
                    test_current = 0,
                    test_total = 0,
                    test_passed_count = 0,
                    test_result = NULL
                WHERE id = ?""",
                (now, host_id,)
            )
        else:
            result = conn.execute(
                "UPDATE hosts SET pending_command = ?, command_queued_at = ? WHERE id = ?",
                (request_body.command, now, host_id)
            )
        if result.rowcount == 0:
            raise HTTPException(status_code=404, detail="Host not found")
        conn.commit()
    
    # Broadcast SSE event for immediate UI update
    event_data = {
        "host_id": host_id,
        "pending_command": None if request_body.command == "stop" else request_body.command,
    }
    if request_body.command == "test":
        event_data.update({
            "test_running": True,
            "test_current": 0,
            "test_total": 0,
        })
    elif request_body.command == "stop":
        event_data.update({
            "test_running": False,
            "status": "ok",
        })
    
    await broadcast_event("command_queued", event_data)
    
    return {"status": "queued", "command": request_body.command}


@app.get("/api/hosts/{host_id}/logs")
async def get_logs(host_id: str, request: Request, limit: int = 20):
    """Get command logs for a host."""
    host_id = validate_host_id(host_id)
    require_auth(request)
    
    with get_db() as conn:
        rows = conn.execute("""
            SELECT * FROM command_log
            WHERE host_id = ?
            ORDER BY created_at DESC
            LIMIT ?
        """, (host_id, min(limit, 100))).fetchall()
        return {"logs": [dict(row) for row in rows]}


@app.delete("/api/hosts/{host_id}")
async def delete_host(host_id: str, request: Request):
    """Delete a host from the fleet. Requires session auth + CSRF token."""
    host_id = validate_host_id(host_id)
    require_auth(request)
    
    # Validate CSRF token from header
    session_token = get_session_token(request)
    csrf_token = request.headers.get("X-CSRF-Token", "")
    if not verify_csrf_token(session_token, csrf_token):
        logger.warning(f"CSRF validation failed for delete host {host_id}")
        raise HTTPException(status_code=403, detail="Invalid CSRF token")
    
    logger.info(f"Deleting host: {host_id}")
    
    with get_db() as conn:
        # Check if host exists
        existing = conn.execute("SELECT id FROM hosts WHERE id = ?", (host_id,)).fetchone()
        if not existing:
            raise HTTPException(status_code=404, detail="Host not found")
        
        # Delete command logs first (foreign key)
        conn.execute("DELETE FROM command_log WHERE host_id = ?", (host_id,))
        # Delete the host
        conn.execute("DELETE FROM hosts WHERE id = ?", (host_id,))
        conn.commit()
    
    return {"status": "deleted", "host_id": host_id}


# ============================================================================
# Web UI
# ============================================================================


@app.get("/", response_class=HTMLResponse)
async def dashboard(request: Request):
    """Render the dashboard."""
    token = get_session_token(request)
    if not token or not verify_session(token):
        return RedirectResponse(url="/login", status_code=302)

    # Get nixcfg info (source of truth) for header
    nixcfg_hash, nixcfg_message = get_nixcfg_info()
    latest_hash = nixcfg_hash  # For host outdated comparison
    
    # Get NixFleet build hash for footer
    build_hash = get_build_hash()
    
    with get_db() as conn:
        rows = conn.execute("""
            SELECT * FROM hosts ORDER BY
                CASE criticality WHEN 'high' THEN 1 WHEN 'medium' THEN 2 ELSE 3 END,
                hostname
        """).fetchall()
        
        hosts = []
        for row in rows:
            host = dict(row)
            
            # Calculate online status and add ISO timestamps for JS
            if host["last_seen"]:
                last_seen = datetime.fromisoformat(host["last_seen"])
                host["online"] = datetime.utcnow() - last_seen < timedelta(minutes=5)
                host["last_seen_relative"] = relative_time(last_seen)
                host["last_seen_iso"] = host["last_seen"]  # ISO format for JS
            else:
                host["online"] = False
                host["last_seen_relative"] = "never"
                host["last_seen_iso"] = None
            
            # Format other times with ISO for tooltips
            if host["last_switch"]:
                host["last_switch_relative"] = relative_time(
                    datetime.fromisoformat(host["last_switch"])
                )
            else:
                host["last_switch_relative"] = "never"
            
            if host["last_audit"]:
                host["last_audit_relative"] = relative_time(
                    datetime.fromisoformat(host["last_audit"])
                )
                host["last_audit_iso"] = host["last_audit"]
            else:
                host["last_audit_relative"] = None
                host["last_audit_iso"] = None
            
            # Command timeout: if pending for >5 min, mark as stale
            COMMAND_TIMEOUT_MINUTES = 5
            if host.get("pending_command") and host.get("command_queued_at"):
                queued_at = datetime.fromisoformat(host["command_queued_at"])
                if datetime.utcnow() - queued_at > timedelta(minutes=COMMAND_TIMEOUT_MINUTES):
                    # Command timed out - clear it
                    host["pending_command"] = None
                    host["test_running"] = 0
                    # Also update in DB
                    conn.execute(
                        "UPDATE hosts SET pending_command = NULL, test_running = 0 WHERE id = ?",
                        (host["id"],)
                    )
                    conn.commit()
            
            # Test state - convert SQLite int to Python bool
            host["test_running"] = bool(host.get("test_running", 0))
            host["test_current"] = host.get("test_current", 0)
            host["test_total"] = host.get("test_total", 0)
            host["test_passed_count"] = host.get("test_passed_count", 0)
            host["test_total_count"] = host.get("test_total", 0)
            host["test_passed"] = (host["test_passed_count"] == host["test_total_count"]) if host["test_total_count"] > 0 else None
            host["poll_interval"] = host.get("poll_interval", 30)
            
            # Parse metrics JSON if present
            metrics_str = host.get("metrics")
            if metrics_str:
                try:
                    host["metrics"] = json.loads(metrics_str)
                except (json.JSONDecodeError, TypeError):
                    host["metrics"] = None
            else:
                host["metrics"] = None
            
            # Check if host is outdated vs GitHub
            host_gen = host.get("current_generation")
            if host_gen and latest_hash:
                host["outdated"] = not latest_hash.startswith(host_gen[:7]) and not host_gen.startswith(latest_hash[:7])
            else:
                host["outdated"] = False
            host["latest_hash"] = latest_hash[:7] if latest_hash else None
            
            hosts.append(host)

    # Calculate stats
    stats = {
        "total": len(hosts),
        "online": sum(1 for h in hosts if h["online"]),
        "audited": sum(1 for h in hosts if h.get("last_audit")),
    }

    # Get CSRF token for this session
    csrf_token = get_csrf_token(token) or ""
    
    return render_template(
        "dashboard.html", 
        hosts=hosts, 
        stats=stats, 
        version=VERSION,
        # nixcfg info for header (source of truth)
        nixcfg_hash=nixcfg_hash[:7] if nixcfg_hash else None,
        nixcfg_message=nixcfg_message or "",
        # For host outdated comparison
        latest_hash=latest_hash[:7] if latest_hash else None,
        # NixFleet build hash for footer
        build_hash=build_hash[:7] if build_hash else None,
        csrf_token=csrf_token,
        action_lock_max_seconds=UI_ACTION_LOCK_MAX_SECONDS,
        csp_nonce=getattr(request.state, "csp_nonce", ""),
    )


def relative_time(dt: datetime) -> str:
    """Format datetime as relative time string."""
    diff = datetime.utcnow() - dt
    seconds = diff.total_seconds()
    
    if seconds < 60:
        return "just now"
    elif seconds < 3600:
        return f"{int(seconds / 60)} min ago"
    elif seconds < 86400:
        return f"{int(seconds / 3600)}h ago"
    else:
        return f"{int(seconds / 86400)}d ago"
