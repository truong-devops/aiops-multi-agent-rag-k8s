"""GitLab CI evidence collector placeholder."""


class GitLabCollector:
    """Collect pipeline status, failed jobs and commit metadata."""

    async def collect(self, incident_id: str) -> list[dict]:
        return [{"source": "gitlab", "incident_id": incident_id, "items": []}]
