"""Kubernetes evidence collector placeholder."""


class KubernetesCollector:
    """Collect pod status, events and workload metadata for an incident."""

    async def collect(self, incident_id: str) -> list[dict]:
        return [{"source": "kubernetes", "incident_id": incident_id, "items": []}]
