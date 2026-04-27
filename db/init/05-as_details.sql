CREATE TABLE as_details (
    ip INET NOT NULL PRIMARY KEY,
    asn INT NULL,
    name TEXT NULL,
    cc TEXT NULL,
    bgp_prefix TEXT NULL
);

CREATE INDEX idx_as_details_asn ON as_details (asn);