# T-019 — WebSocket-сессии и ограничения транспорта

Проверено 2026-11-21. Hysen Labs содержит карточку Coder WebSocket. Поиск по Go WebSocket использован для обнаружения проекта. Свойства точных тегов и активность сверены с upstream, а выбор Coder уже одобрен владельцем. [Каталог](https://hysenlabs.com/projects) · [Карточка Coder](https://hysenlabs.com/projects/coder-websocket) · [Решение T-003](go-websocket.md)

| Вариант | Лицензия, активность и применимость |
|---|---|
| Coder WebSocket v1.8.15 | ISC, push 2026-11-20. Go 1.23, нет внешних модулей. `Accept`, переговоры о subprotocol, лимиты чтения и контекстные операции подходят для D-003. Включение одного пина соответствует T-019. [GitHub](https://github.com/coder/websocket/tree/v1.8.15) · [Документация](https://pkg.go.dev/github.com/coder/websocket@v1.8.15) |
| Gorilla WebSocket v1.5.3 | BSD-2-Clause, push 2025-03-19, Go 1.12, нет внешних модулей. Пригоден с одним reader, одним writer и явным управлением deadlines, но это резерв без одобрения для T-019. [GitHub](https://github.com/gorilla/websocket/tree/v1.5.3) |
| gobwas/ws v1.4.0 | MIT, push 2026-11-10, Go 1.16, три требования в файле модуля. Низкоуровневый доступ полезен для специализированного транспорта. Для среза владелец выбрал готовое управление кадрами и контекстами Coder. [GitHub](https://github.com/gobwas/ws/tree/v1.4.0) |

`Accept` выбирает пересечение подпотоколов, но разрешает пустой результат. Серверная сторона проверяет предложение `newmmo.v1` до 101 и отключает compression. WebSocket получает соединение через `Hijack`; у HTTP logging wrapper уже есть этот метод. После upgrade соединение живёт на собственном контексте, а дедлайны HTTP должны быть сброшены на захваченном сокете. Стандартный `http.Server.Shutdown` не ждёт его автоматически. [Accept](https://github.com/coder/websocket/blob/v1.8.15/accept.go) · [http.Hijacker](https://pkg.go.dev/net/http#Hijacker) · [Shutdown](https://pkg.go.dev/net/http#Server.Shutdown)

`Ping` ждёт Pong, который обрабатывает активный reader. Истечение контекста чтения или записи может закрыть соединение в самой библиотеке, поэтому для pre-auth и дренажа нужен независимый срок жизни. `Close` ограничивает отправку и ожидание ответа своими таймаутами; общий бюджет завершения должен дополнительно ограничивать захваченный сокет. [Conn](https://github.com/coder/websocket/blob/v1.8.15/conn.go) · [Close](https://github.com/coder/websocket/blob/v1.8.15/close.go)

Реестр учитывает pre-auth и авторизованные соединения. Замена сессии делается под блокировкой по ID персонажа, а закрытие прежнего сокета — после выхода из неё. У каждого сокета одна ограниченная очередь прикладных сообщений и один writer. При переполнении очередь переводит сокет в состояние закрытия с 1013, пока остальные соединения продолжают работу. Динамические данные клиента не попадают в close reason и сообщения журнала. [D-003 §4 и §8](../contracts/websocket-protocol.md) · [RFC 6455](https://www.rfc-editor.org/rfc/rfc6455)

## Уточнения реализации

- Промпт приводит пустой массив quests, тогда как форма снапшота в контракте задаёт объект из трёх массивов. Клиентский smoke использует объект. [Промпт](../director/prompts/T-019.md) · [Контракт снапшота](../contracts/quest-engine.md)
- §10 принимает EX-P04 целиком и включает повторную награду. Промпт T-019 прямо оставляет мутации и идемпотентность T-020. [Критерии D-003](../contracts/websocket-protocol.md) · [План задач](../director/TASKS.md)
- Для трёх состояний `content.version.compatibility` нужно закрепить сравнение минимальной версии, отсутствующего ID и состояния без опубликованного билда. [D-003](../contracts/websocket-protocol.md) · [Экспорт](../contracts/content-export.md)
