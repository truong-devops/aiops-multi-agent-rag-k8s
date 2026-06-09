"""Prometheus evidence collector placeholder."""


class PrometheusCollector:
    """Collect metrics such as latency, 5xx, restarts, memory and queue lag."""

    async def collect(self, incident_id: str) -> list[dict]:
        return [{"source": "prometheus", "incident_id": incident_id, "items": []}]
