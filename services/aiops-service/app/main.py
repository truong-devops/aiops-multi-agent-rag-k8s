from fastapi import FastAPI

app = FastAPI(title="AIOps RCA Service", version="0.1.0")


@app.get("/healthz")
def healthz() -> dict[str, str]:
    return {"status": "ok"}


@app.get("/readyz")
def readyz() -> dict[str, str]:
    return {"status": "ready"}


@app.get("/metrics")
def metrics() -> str:
    return "# metrics placeholder\n"
