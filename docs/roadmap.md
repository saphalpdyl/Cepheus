## Initial plan

- Basic Paris/Dublin traceroute based hop information gathering 
- TWAMP/STAMP reflectors at ==3 different geographical regions 
- Emulate the containerlab infrastructure with modularity in mind ( the end product should be able to replace the emulated part with real network infrastructure )
- Spec-first design (OpenAPI) 
- Modularized end-products ( must be independently deployable ):
    - probe-agent ( TWAMP/traceroute )
    - cepheus-agent ( configuration plane, pull-based configuration from be )
    - backend ( control plane for configuration, probe agent on/off on every cepheus-agent based node )
    - frontend (next.js)
- Time series based database setup

### Architectural decisions
- Cepheus-agent sits inside the managed network and on different regions. It will use the `cepheus-stamp` library to act both as `Session-Sender` & `Session-Reflector` as defined in RFC 8762