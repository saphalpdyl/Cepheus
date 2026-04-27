CREATE TABLE cloud_ip_ranges_meta (
--     Defines which service it is AWS, GCP, Cloudflare etc.
    provider     TEXT PRIMARY KEY,
    last_updated TIMESTAMPTZ NOT NULL DEFAULT NOW()
);


CREATE TABLE cloud_ip_ranges (
    id BIGSERIAL PRIMARY KEY,
    prefix CIDR NOT NULL,

--     Defines which service it is AWS, GCP, Cloudflare etc.
    provider TEXT NOT NULL,
    data JSONB NOT NULL
)