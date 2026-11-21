# T-018 — поправка №1: Go 1.26.11 (после `govulncheck`)

`govulncheck` нашёл 9 достижимых уязвимостей стандартной библиотеки (net/http, crypto/tls, crypto/x509, net/textproto) в **Go 1.25.14**; исправления есть только в **Go ≥ 1.26.9**. Сервер переведён на **Go 1.26.11**: в `main` теперь `server/go.mod` = `go 1.26.0` + `toolchain go1.26.11`, CI использует `setup-go 1.26.x`. На 1.26.11 `govulncheck ./...` — «No vulnerabilities found».

Что сделать в ветке `agent/server/T-018` (до сдачи отчёта):
1. `git fetch origin && git rebase origin/main` (или merge main в свою ветку, как делал раньше) — подтяни новый `go.mod`.
2. Локально используй Go 1.26.11 (`GOTOOLCHAIN=auto` скачает; `go version` должен показать go1.26.11). **Не возвращай `go 1.25.0` в `go.mod`**, других изменений в `go.mod` по этой поправке не нужно.
3. Прогони заново `go vet ./...`, `go test ./... -race -count=1` (с БД), `go mod verify` и `govulncheck ./...` (`GOTOOLCHAIN=auto go install golang.org/x/vuln/cmd/govulncheck@v1.10.0`) — в отчёте приведи вывод на 1.26.11.
4. Если твой код вызывает новую уязвимость в зависимостях (`govulncheck` покажет) — не заглушай, а сообщи в отчёте.
Остальное задание T-018 не меняется.
