"""Log agent placeholder."""


class LogAgent:
    """Analyze log evidence and runbook log patterns."""

    async def run(self, evidence: list[dict]) -> dict:
        return {"agent": "log", "evidence_count": len(evidence), "findings": []}
