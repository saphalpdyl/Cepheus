# Project Roadmap

This document covers the roadmaps, with emphasis on the argus detection engine.

## Argus Milestones
I have decided to place the detection engine's development in the form of three distinct milestones, each improving and adding features to the previous milestone.

### Milestone I
Complete anomaly detection between cloud and edge agents on STAMP, PING metrics as well as minimal detection on traceroute path changes based on its ASN traversal fingerprint and link traversal fingerprint ( different granularity ). Use STAMP between owned agents and ping where ownership is absent.

### Milestone II
Reasonable per-hop/per-link fault attribution in trace paths accompanied by correlation with end-to-end measurements ( STAMP and PING ). Confidence on detection will only be available to the last mile ISP or target's last mile paths due to probe diversity there from the agent mesh network.

### Milestone III
The criteria for completion of milestone III has not been solidified yet. At surface-level, milestone III should take the hop-attribution of milestone II and fill in the missing network tomography in the in-transit hops.

We can approach this milestone as incooperating data from RIPE Stat and our own agent trace data to increase probe diversity in the in-transit links to get a good confidence detection. Furthermore, if Cepheus becomes adopted by the community, we can crowd-source anonymized data, on the user's consent, and give back to RIPE state benefiting both sides.
