"""Loki evidence collector placeholder."""


class LokiCollector:
    """Collect service logs and error patterns around an incident time window."""

    async def collect(self, incident_id: str) -> list[dict]:
        return [{"source": "loki", "incident_id": incident_id, "items": []}]
