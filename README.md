<img src="docs/images/logo.png" width="500">

---

> [!IMPORTANT]
> The repository on GitHub is a mirror maintained for visibility. Issue tracking and active development are done on [GitLab.](https://gitlab.com/saphalpdyl/Cepheus)

Modularized route health monitoring with software-based agents for managed network systems.

[![License](https://img.shields.io/badge/License-Apache%202.0-blue.svg)](https://opensource.org/licenses/Apache-2.0)
[![Pipeline status](https://gitlab.com/saphalpdyl/Cepheus/badges/main/pipeline.svg)](https://gitlab.com/saphalpdyl/Cepheus/-/commits/main)

> Created and Maintained by [Saphal K. Poudyal](https://saphal.me)

### Table of Contents

- [ROADMAP](/roadmap.md)
- [Architecture](#architecture)
- [Contributions](#contributions)
- [Setup development environment](/docs/setup-dev.md)
- [Screenshots](#screenshots--prototype-)

---
### Architecture
The philosophy is that the system components should be independently scalable, deployable, and implementation agnostic.

Given the spec-driven design that will be implementated once the API and requirements stablize, every major component should be replacable were it to conform to the specfications.

Below is the draft architecture diagram for cepheus platform as a whole.

![architecture diagram](/docs/images/arch.svg)

# Contributions & Governance
Cepheus is an independent open-source project create and maintained by Saphal Kumar Poudyal. All architectural decisions, direction of the roadmap, and project governance are at the sole discretion of the maintainer.

Contributions are welcome via pull requests and issues. However, accepted contributions do not transfer governance rights.

### Screenshots
![Fleet Overview](/docs/images/screenshots/redesign/fleet.png)
![Per-device dashboard](/docs/images/screenshots/redesign/per_device.png)
![Alerts Overview](/docs/images/screenshots/redesign/alerts.png)
