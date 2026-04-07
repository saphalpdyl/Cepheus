## Initial plan ( might not make it to the end ) 

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