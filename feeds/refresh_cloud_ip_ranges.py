import psycopg
from psycopg.types.json import Json
import httpx
import os
import re
from dataclasses import dataclass
from datetime import datetime, timedelta, timezone


@dataclass
class Config:
    db_url: str
    max_age: str = "7d"


def get_config() -> Config:
    db_url = os.environ.get("DATABASE_URL")
    if not db_url:
        raise ValueError("DATABASE_URL environment variable is not set.")

    max_age = os.environ.get("MAX_AGE", "7d")
    try:
        parse_duration(max_age)
    except ValueError:
        raise ValueError(f"Invalid MAX_AGE value: {max_age}")
    return Config(db_url=db_url, max_age=max_age)


def parse_duration(s: str) -> timedelta:
    units = {
        's':   'seconds',
        'm':   'minutes',
        'min': 'minutes',
        'h':   'hours',
        'd':   'days',
        'w':   'weeks',
    }
    match = re.fullmatch(r'(\d+)\s*(w|d|h|min|m|s)', s.strip())
    if not match:
        raise ValueError(f"Invalid duration: {s}")
    value, unit = int(match.group(1)), match.group(2)
    return timedelta(**{units[unit]: value})


def is_stale(last_updated: datetime, max_age: str) -> bool:
    return last_updated + parse_duration(max_age) < datetime.now(timezone.utc)


def fetch_aws_ranges() -> list[tuple]:
    data = httpx.get("https://ip-ranges.amazonaws.com/ip-ranges.json").json()
    rows = []
    for p in data.get("prefixes", []):
        rows.append((
            p["ip_prefix"],
            "aws",
            Json({"region": p.get("region"), "service": p.get("service"), "network_border_group": p.get("network_border_group")})
        ))
    for p in data.get("ipv6_prefixes", []):
        rows.append((
            p["ipv6_prefix"],
            "aws",
            Json({"region": p.get("region"), "service": p.get("service"), "network_border_group": p.get("network_border_group")})
        ))
    return rows


def fetch_gcp_ranges() -> list[tuple]:
    data = httpx.get("https://www.gstatic.com/ipranges/cloud.json").json()
    rows = []
    for p in data.get("prefixes", []):
        prefix = p.get("ipv4Prefix") or p.get("ipv6Prefix")
        if prefix:
            rows.append((
                prefix,
                "gcp",
                Json({"scope": p.get("scope"), "service": p.get("service")})
            ))
    return rows


PROVIDERS = {
    "aws": fetch_aws_ranges,
    "gcp": fetch_gcp_ranges,
}


def refresh_provider(conn: psycopg.Connection, provider: str, max_age: str):
    with conn.cursor() as cur:
        cur.execute(
            "SELECT last_updated FROM cloud_ip_ranges_meta WHERE provider = %s",
            (provider,)
        )
        row = cur.fetchone()

        if row is not None and not is_stale(row[0], max_age):
            print(f"[{provider}] still fresh, skipping.")
            return

        print(f"[{provider}] refreshing...")

        fetch_fn = PROVIDERS[provider]
        rows = fetch_fn()

        with conn.transaction():
            cur.execute("DELETE FROM cloud_ip_ranges WHERE provider = %s", (provider,))
            cur.executemany(
                "INSERT INTO cloud_ip_ranges (prefix, provider, data) VALUES (%s, %s, %s)",
                rows
            )
            cur.execute(
                """
                INSERT INTO cloud_ip_ranges_meta (provider, last_updated)
                VALUES (%s, NOW())
                ON CONFLICT (provider) DO UPDATE SET last_updated = NOW()
                """,
                (provider,)
            )

        print(f"[{provider}] loaded {len(rows)} prefixes.")


def main():
    try:
        config = get_config()
    except ValueError as e:
        print(f"Configuration error: {e}")
        return

    with psycopg.connect(config.db_url) as conn:
        for provider in PROVIDERS:
            refresh_provider(conn, provider, config.max_age)


if __name__ == "__main__":
    main()