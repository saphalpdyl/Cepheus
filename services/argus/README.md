# Cepheus Argus
Argus, dervied from the greek mythology of a "all-seeing" giant with hundred eyes. 
Argus is the anomaly detection system of Cepheus, that consumes processed, enriched data 
from the data pipeline.

### Detectors
Detectors like EWMA ( Exponentially Weighted Moving Average ) do , by convention,
findings reporting. When anamalous change is detected, the detector simply reports
the finding to the upper layer

### Policy Engine
This is the core of argus that handles reporting and possibly will autonomously manage
detector config ( cadence, alpha values etc. ). It consumes findings and based on 
policies described ( **TODO**: from the control plane ) and generates
alarms and events.

### Runner
This is equivalent to a simplified version of the supervisor from `agent`, as in it does
not reconcile against the control plane config. The runner is responsible for starting the 
detectors on configured cadence and manages the data streames between the detector and the policy engine.
