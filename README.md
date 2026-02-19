# Mini Discord

Сервис в формате mini-discord, реализованный на Go:
- текстовые каналы в реальном времени;
- история сообщений в SQLite;
- голосовые каналы на WebRTC;
- TURN сервер через coturn для стабильного соединения.

## Запуск

Перед запуском убедитесь, что в корне проекта есть файл `.env`. Пример конфигурации находится в `.env.example`.

### Локальный запуск приложения (через docker-compose)

1. Клонирование репозитория
```bash
git clone <your-repo-url>
cd discord
```

2. Подготовка окружения
```bash
cp .env.example .env
```

3. Запуск приложения
```bash
docker compose up --build
```

После выполнения команды будут подняты контейнеры:

| Сервис          | Имя контейнера      | Доступный порт на хосте             |
|-----------------|---------------------|-------------------------------------|
| Основной API    | mini-discord-app    | localhost:8000                      |
| TURN (coturn)   | mini-discord-turn   | localhost:3478 (tcp/udp), relay udp |

4. Остановка контейнеров
```bash
docker compose down -v
```

### Локальный запуск приложения (без Docker)

1. Запуск приложения
```bash
go run ./cmd/server
```

2. Открыть приложение
```text
http://localhost:8000
```

## Реализация

1. **Текстовый чат реализован через WebSocket**:
   - endpoint: `/ws`;
   - поддержка подключения/отключения клиентов;
   - broadcast сообщений в реальном времени;
   - обновление online presence.

2. **История сообщений хранится в SQLite**:
   - таблица `messages` создается автоматически при старте;
   - история доступна через `GET /api/history?channel=<name>&limit=<n>`;
   - данные сохраняются в `data/chat.db`.

3. **Голосовые каналы реализованы на WebRTC**:
   - media передается peer-to-peer через `RTCPeerConnection`;
   - signaling идет через WebSocket endpoint `/ws/voice`;
   - поддержан обмен `offer/answer/candidate`.

4. **TURN/STUN конфигурация вынесена в API**:
   - endpoint: `GET /api/webrtc-config`;
   - клиент получает `iceServers` динамически;
   - fallback по умолчанию: `stun:stun.l.google.com:19302`.

5. **TURN сервер подключен через coturn (Docker)**:
   - конфиг: `deploy/turnserver.conf`;
   - параметры доступа задаются через `.env`;
   - используется для соединений, где прямой P2P недоступен.

## Что изучено в проекте

- Практическая работа с WebSocket на backend/frontend.
- Проектирование signaling-слоя для WebRTC.
- Полный WebRTC-флоу: `offer -> answer -> ICE candidate`.
- Настройка STUN/TURN для голосовой связи.
- Dockerization backend + coturn сервиса.
- Работа с SQLite в Go и модульной структурой проекта.

## Вопросы и решения

### 1. Почему голос сначала был на WebSocket, а потом на WebRTC?

**Проблема:** WebSocket удобен для signaling и чата, но плохо подходит как основной транспорт для голоса в реальном времени.  
Нужно вручную решать буферизацию, сетевую адаптацию и устойчивость audio-потока.

**Решение:** Перевел voice-каналы на WebRTC, а WebSocket оставил только для signaling.  
Это дало более стабильную голосовую связь и архитектурно корректный подход для RTC-задач.

### 2. Зачем нужен TURN, если уже есть STUN?

**Проблема:** STUN не всегда помогает установить прямое соединение (например, строгие NAT/фаерволы).  

**Решение:** Добавлен TURN (coturn), который работает как relay и позволяет установить связь даже в сложных сетевых условиях.

## Переменные окружения

Пример в `.env.example`:

```env
TURN_HOST=localhost
TURN_PORT=3478
TURN_USERNAME=webrtc
TURN_PASSWORD=webrtcpass
```

Для production:
- укажи публичный IP/домен в `TURN_HOST`;
- замени `TURN_PASSWORD` на безопасный пароль;
- при необходимости включи TLS для coturn.
