"""Argo CD evidence collector placeholder."""


class ArgoCDCollector:
    """Collect sync status, revision and rollout history."""

    async def collect(self, incident_id: str) -> list[dict]:
        return [{"source": "argocd", "incident_id": incident_id, "items": []}]
