"""Planner agent placeholder."""


class PlannerAgent:
    """Select specialist agents and assemble the final RCA report."""

    async def run(self, incident: dict) -> dict:
        return {"agent": "planner", "incident_id": incident.get("incident_id"), "findings": []}
