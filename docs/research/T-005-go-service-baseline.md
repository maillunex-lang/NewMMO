# T-005 — HTTP-база для Go dev-сервера

Проверено 2026-11-21. Для текущей заглушки нужны конфигурация из окружения, раздельные проверки жизни и готовности, ограниченное по времени завершение и структурные логи. Целевая версия модуля — Go 1.22.

Поиск в каталоге Hysen Labs по `go http` дал карточки chi и Gin, а запрос `health-go` — карточку одноимённой библиотеки. Лицензии и активность проверены через GitHub, требования к Go и зависимости — в `go.mod` соответствующих тегов. [Каталог Hysen Labs](https://hysenlabs.com/projects) · [Поиск Go HTTP](https://hysenlabs.com/search?q=go%20http) · [Поиск health-go](https://hysenlabs.com/search?q=health-go)

| Вариант | Лицензия и активность | Go и зависимости | Пригодность для EX-P04/P05 |
|---|---|---|---|
| Go stdlib | BSD-3-Clause, push 2026-11-21 | Все нужные API доступны в Go 1.22. Дополнительных модулей 0. | `http.Handler` подходит для будущего HTTP-входа в WebSocket. Жизненный цикл сессий и авторитетная симуляция остаются задачами сервера. [GitHub](https://github.com/golang/go) · [История релизов Go](https://go.dev/doc/devel/release) |
| chi | MIT, push 2026-11-07 | v5.5.0 требует Go 1.24. v5.2.2 требует Go 1.20. У обоих тегов 0 требований в `go.mod`, при подключении добавился бы один модуль chi. | Совместим с `http.Handler` и удобен при появлении групп маршрутов. Для двух проб встроенного `ServeMux` достаточно. [Hysen Labs](https://hysenlabs.com/projects/go-chi-chi) · [GitHub](https://github.com/go-chi/chi) · [v5.5.0](https://github.com/go-chi/chi/blob/v5.5.0/go.mod) · [v5.2.2](https://github.com/go-chi/chi/blob/v5.2.2/go.mod) |
| Gin | MIT, push 2026-10-10 | v1.12.0 требует Go 1.25.0. В `go.mod` 15 прямых и 20 косвенных требований, включая зависимости тестов. v1.10.1 требует Go 1.20 и содержит 12 прямых и 17 косвенных требований. | Можно подключить WebSocket через HTTP-обработчик. Контекст Gin, привязка JSON и валидаторы расширяют поверхность поддержки без пользы для текущих проб и игрового цикла. [Hysen Labs](https://hysenlabs.com/projects/gin-gonic-gin) · [GitHub](https://github.com/gin-gonic/gin) · [v1.12.0](https://github.com/gin-gonic/gin/blob/v1.12.0/go.mod) · [v1.10.1](https://github.com/gin-gonic/gin/blob/v1.10.1/go.mod) |
| health-go | Apache-2.0, push 2026-11-09 | v5.5.5 требует Go 1.23.0. В `go.mod` 16 прямых и 38 косвенных требований, включая зависимости тестов и готовых интеграций. | Полезен для агрегирования проверок внешних сервисов. Для текущей готовности достаточно состояния процесса. WebSocket и игровые сессии обслуживаются отдельно. [Hysen Labs](https://hysenlabs.com/projects/hellofresh-health-go) · [GitHub](https://github.com/hellofresh/health-go) · [v5.5.5](https://github.com/hellofresh/health-go/blob/v5.5.5/go.mod) |

Числа описывают объявления в файлах модулей, а фактический набор пакетов в бинарнике зависел бы от импортов. Таблица оценивает серверную часть. Godot и Android будут взаимодействовать с ней по сетевому контракту.

## Выбор для T-005

- Читать `NEWMMO_ADDR` при запуске. Если значение пустое, использовать `PORT` на `127.0.0.1`, затем `127.0.0.1:18086` как значение по умолчанию. Ошибку открытия listener возвращать как ошибку запуска. [os.Getenv](https://pkg.go.dev/os#Getenv) · [net.Listen](https://pkg.go.dev/net#Listen)
- Сохранить поля `ok`, `service`, `time` в `/healthz`. Готовность определяется тем, принимает ли процесс работу, и становится отрицательной с момента получения сигнала завершения. Выставить явные таймауты заголовка, чтения, записи и keep-alive. [http.Server](https://pkg.go.dev/net/http#Server)
- Использовать `signal.NotifyContext` для SIGINT/SIGTERM. Для `Shutdown` создать отдельный контекст с таймаутом, дождаться активных HTTP-запросов и закрыть соединения при исчерпании бюджета. Для будущих WebSocket нужен собственный учёт сессий и завершение соединений: `Shutdown` не ждёт hijacked-соединения. [os/signal.NotifyContext](https://pkg.go.dev/os/signal#NotifyContext) · [context.WithTimeout](https://pkg.go.dev/context#WithTimeout) · [http.Server.Shutdown](https://pkg.go.dev/net/http#Server.Shutdown)
- Писать JSON через `slog`. Генерировать идентификатор на сервере, возвращать его в `X-Request-ID` и записывать вместе со статусом и длительностью. Список полей исключает URI, query string, входные заголовки и тела. [slog.NewJSONHandler](https://pkg.go.dev/log/slog#NewJSONHandler)

Итог — использовать stdlib. Подключение внешних компонентов для T-005 не требуется. Выбор WebSocket-библиотеки остаётся частью отдельного исследования T-003 и решения владельца.
