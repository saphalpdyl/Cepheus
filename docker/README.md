# Docker files

This `/docker/` directory contains all the Dockerfile required by Cepheus and tests. The `dev/` directory contains dockerfiles required for development or Containerlab emulations such as `security-appliance` node, which requires `security-appliance.Dockerfile`. 

The `tests/` directory contains dockerfiles required for integration/e2e testing.

### Conventions
- `*.build.Dockerfile` are Dockerfile that are used to build binaries-- `cepheus-agent`, `scamper` etc.