.PHONY: db api web test-api test-web qa schema smoke

db:
	docker compose up -d postgres
api:
	cd api && go run .
web:
	cd web && npm run dev
test-api:
	cd api && go test ./...
test-web:
	cd web && npm run typecheck && npm run lint && npm run build && npm run test
schema:
	psql "$$DATABASE_URL" -v ON_ERROR_STOP=1 -f db/schema.sql
smoke:
	psql "$$DATABASE_URL" -v ON_ERROR_STOP=1 -f db/smoke.sql
qa:
	./scripts/run-qa.sh
