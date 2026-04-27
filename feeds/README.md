# Enrichment: Data Processing Pipeline helper

Here includes the scripts ( usually in python ) that are invoked by a cron job,
or on dev script start to refresh lookup tables for enrichment stage
of the data processing pipeline.

### Cloud Provider IP Ranges
[refresh_cloud_ip_ranges.py](enrichment/refresh_cloud_ip_ranges.py) refreshes the local ip-ranges caches from different
cloud service providers. AWS and GCP are supported for now. 

#### Sources
AWS: https://ip-ranges.amazonaws.com/ip-ranges.json

GCP: https://www.gstatic.com/ipranges/cloud.json

These are used for hop enrichment when doing path visualization in the frontend.
`as_details.ip` is used as the lookup for the prefix in the `cloud_ip_ranges` table.
