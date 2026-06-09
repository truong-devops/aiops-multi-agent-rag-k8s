"""Metric agent placeholder."""


class MetricAgent:
    """Analyze metric anomalies and time correlation."""

    async def run(self, evidence: list[dict]) -> dict:
        return {"agent": "metric", "evidence_count": len(evidence), "findings": []}
