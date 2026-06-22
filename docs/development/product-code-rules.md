# Product Code Rules

Tai lieu nay dinh nghia cac nguyen tac code phan san pham theo huong production-grade cho du an nay. Muc tieu la de khi phat trien feature, moi service van giu duoc tinh nghiem tuc, co kha nang deploy len moi truong that, va khong pha vo cau truc tong the cua he thong.

Tai lieu nay ap dung chu yeu cho:

- `services/api-gateway`
- `services/identity-service`
- `services/video-service`
- `services/media-worker`
- `services/feed-social-service`
- `services/live-service`
- `apps/admin-web` va cac client product ve sau

Khong ap dung nhu tai lieu chinh cho logic RCA cua `aiops-service`, du service do van phai ton trong boundary va convention chung.

## 1. Muc tieu thiet ke

Code san pham trong repo nay phai huong toi 4 muc tieu:

1. Chay duoc tren local, dev, demo va Kubernetes.
2. Co boundary ro rang giua cac service de tranh hidden coupling.
3. Co kha nang quan sat, debug va truy vet su co khi deploy that.
4. Don gian vua du de phuc vu khoa luan, nhung khong lam theo kieu demo tam bo.

Neu co nhieu cach lam, uu tien cach:

- ro ownership
- de test
- de observability
- de mo rong sau nay
- it magic

## 2. Nguyen tac tong quat khi code feature

- Moi feature phai thuoc ro mot bounded context.
- Khong viet feature theo kieu "di xuyen nhieu service" ma khong co contract ro rang.
- Khong dua business logic vao `api-gateway`.
- Khong doc database cua service khac.
- Khong xem Redis la source of truth.
- Khong su dung state ngam suy ra tu log neu state can ton tai trong database.
- Khong code nhanh theo kieu hard-code local roi de sau sua.

Moi thay doi nen tra loi duoc 3 cau hoi:

1. Feature nay thuoc service nao?
2. Du lieu canonical nam o dau?
3. Evidence van hanh nao duoc tao ra khi feature nay chay that?

## 3. Rule ve to chuc code trong service

Moi service can duoc to chuc de de doc, de test, de thay ownership.

Huong uu tien:

- `cmd/server` chua diem khoi dong
- `internal/config` chua config va validation
- `internal/handler` chua HTTP transport layer
- `internal/service` chua business use case
- `internal/repository` chua persistence access
- `internal/domain` chua model, invariant, error domain
- `internal/observability` chua middleware, metrics, logging helpers

Nguyen tac:

- Handler khong chua business logic phuc tap.
- Repository khong chua policy nghiep vu.
- Service khong bind chat vao HTTP framework hay database driver neu khong can.
- Domain error phai ro rang va map duoc sang API error code hop ly.

Neu service nho, co the chua can tach het ngay, nhung van phai giu huong tach lop ro rang.

## 4. Rule ve API

Tat ca public API phai di qua `api-gateway` duoi dang `/api/v1/*`.

Tat ca internal product service API nen:

- dung `/v1/*`
- dung plural noun
- dung snake_case cho JSON field
- co response envelope thong nhat
- co error code on dinh
- ton trong `X-Request-ID` va `X-Correlation-ID`

Khi them API moi:

- xac dinh owner service
- xac dinh input/output schema
- xac dinh validation rule
- xac dinh error code
- xac dinh idempotency neu la `POST` co the bi retry
- xac dinh log nao duoc ghi va log nao phai redact

Khong expose endpoint "tam" chi de test neu no lam vo API surface. Neu can endpoint operational, dat ro muc dich, vi du `/healthz`, `/readyz`, `/metrics`.

## 5. Rule ve du lieu va persistence

Can follow `docs/architecture/database-design.md`.

Nguyen tac bat buoc:

- Moi service so huu schema/database cua rieng no.
- Chi luu cross-service reference bang ID.
- PostgreSQL la noi luu canonical cho giao dich va lifecycle state.
- MongoDB chi dung cho document linh hoat dung theo thiet ke.
- Redis chi dung cho cache, rate limit, lock, counter, idempotency ngan han.

Moi bang/trang thai nghiep vu quan trong nen co:

- `id`
- `created_at`
- `updated_at`
- `status` neu co state machine
- `error_code` neu co failure state
- `request_id` hoac `correlation_id` khi can truy vet

Khong luu:

- raw password
- raw refresh token
- OAuth code
- presigned URL trong log
- stream key plaintext trong log
- thong tin nhay cam khong can thiet cho nghiep vu

## 6. Rule ve state machine

Nhung module co lifecycle phai code theo explicit state transition.

Toi thieu:

- video: `draft -> uploaded -> processing -> ready | failed | deleted`
- processing job: `queued -> running -> succeeded | retrying | failed | dead_letter`
- live session: `scheduled -> live -> ended | failed`

Rule:

- Khong nhay state vo ly.
- Moi state transition phai co noi cap nhat ro rang.
- Failure phai co `error_code` on dinh neu co the.
- Neu state thay doi boi worker hay async flow, phai co audit duoc qua event, log, hoac history table.

## 7. Rule ve event-driven flow

Moi giao tiep async giua service phai co event contract ro.

Khi them event moi:

- dat ten event on dinh
- xac dinh producer
- xac dinh consumer
- xac dinh entity ID lien quan
- kem `event_id`, `correlation_id`, `timestamp`
- version duoc neu payload co nguy co thay doi

Khong publish event mo ho hoac payload phu thuoc cau truc noi bo mot cach mong manh.

Neu du lieu cua service khac can cap nhat read model, hay dung event de dong bo, khong doc truc tiep database service do.

## 8. Rule ve security

Code san pham phai mac dinh theo huong an toan.

Nguyen tac:

- Validate input som.
- Hash password dung co che phu hop, khong tu viet crypto.
- JWT/session phai co issuer, audience, TTL ro rang.
- Token va secret khong duoc in ra log.
- Config nhay cam phai di qua environment variable hoac secret manager sau nay.
- OAuth flow phai kiem tra state, nonce, redirect constraint.
- Rate limit cho endpoint nhay cam nhu login, register, refresh khi can.

Khong vi tien local ma bo qua cac diem security can ban neu no anh huong truc tiep den behavior production.

## 9. Rule ve observability

Moi feature production-grade phai co kha nang quan sat.

Bat buoc:

- `healthz`
- `readyz`
- `metrics`
- structured logging
- request ID va correlation ID

Nen co:

- log state transition quan trong
- log retry, dependency error, timeout, queue lag
- metric cho request count, latency, error rate, worker outcome

Log phai huu ich cho RCA:

- co service name
- co environment
- co request/correlation ID khi co
- co entity ID khi co
- khong lo secret

## 10. Rule ve config va environment

Code phai phan tach ro config va business logic.

Moi service nen:

- load config tu environment
- validate config luc startup
- fail fast neu thieu dependency quan trong trong moi truong non-local
- co default hop ly cho local development khi tai lieu cho phep

Khong hard-code:

- URL dependency
- secret
- bucket name neu da co config
- runtime mode
- image-specific assumption

## 11. Rule ve external dependency

Moi dependency ngoai process deu phai duoc xem la co the fail.

Khi goi database, Redis, object storage, queue, hay upstream HTTP:

- dat timeout hop ly
- xu ly retry co kiem soat neu phu hop
- tra ve error ro rang
- log du thong tin debug
- khong swallow error

Neu dependency xuong, service phai fail theo cach du doan duoc. Uu tien "fail clear" hon "chay mo ho".

## 12. Rule ve test

Moi logic quan trong can co test gan nhat co the.

Toi thieu:

- config validation test cho service co config phuc tap
- handler/service test cho API behavior quan trong
- repository test cho truy van quan trong neu co logic query dang ke
- state transition test cho workflow co lifecycle

Khi them feature, can nghi:

- test happy path
- test invalid input
- test conflict/state invalid
- test dependency failure

Neu chua test duoc end-to-end, it nhat phai co test cho use case core.

## 13. Rule ve migration va schema

Bat ky thay doi persistence nao anh huong schema deu phai co migration ro rang.

Nguyen tac:

- migration phai idempotent hoac it nhat co thu tu ro
- ten bang va cot phai ro nghia
- index va constraint quan trong phai duoc khai bao som
- schema phai phan anh domain state chu khong chi phuc vu hien tai tam thoi

Khong sua tay database production-style bang cach khong truy vet duoc.

## 14. Rule ve admin va client app

Admin web va client app la cua ngo vao he thong, nhung khong duoc chi dao architecture backend.

Nguyen tac:

- UI phai goi qua `api-gateway`
- khong dung client de patch logic thay cho backend
- khong goi truc tiep service noi bo khi mo hinh deploy that khong cho phep
- admin page nen uu tien quan sat duoc workflow, health, incident, va processing state

Neu mot tinh nang UI yeu cau backend "hack tam", uu tien sua backend cho dung boundary.

## 15. Rule ve deploy that

Code duoc xem la san sang hon cho moi truong that khi:

- startup fail fast neu config critical thieu
- co health/readiness ro rang
- co logging va metrics co y nghia
- khong phu thuoc local-only shortcut de chay nghiep vu chinh
- co API contract ro
- co migration va persistence rule ro
- co du thong tin de debug khi dependency fail

No khong can dat den muc enterprise hoan hao ngay, nhung phai tranh demo-code kieu:

- tat ca trong `main.go`
- khong validate input
- khong timeout
- khong readiness
- state duoc doan tu log
- hard-code flow chi chay duoc local

## 16. Checklist truoc khi merge mot feature product

Truoc khi xem mot thay doi la "on" cho phan san pham, tu kiem tra:

- Feature nay thuoc dung service chua?
- Boundary co bi vo khong?
- API co dung convention khong?
- State machine co ro khong?
- Du lieu canonical nam dung cho khong?
- Redis co bi dung sai lam source of truth khong?
- Log va metric co du de dieu tra su co khong?
- Config co chay duoc tren local va deploy that khong?
- Co test cho logic chinh khong?
- Co can update docs, contract, migration, hay versioning khong?

## 17. Cach agent nen ap dung tai lieu nay

Neu user yeu cau code phan san pham, agent nen:

1. Doc `AGENTS.md`
2. Doc tai lieu nay
3. Doc boundary, database, API doc lien quan
4. Moi bat dau code

Khi task mo ho, uu tien quyet dinh theo huong:

- production-grade hon
- de deploy that hon
- ro ownership hon
- de observability hon
- it ky thuat "chong chay" hon
