# Uno - System Monitoring CLI

Мощная CLI утилита для мониторинга системы с красивыми TUI интерфейсами.

## Возможности

### 🖥️ Системный мониторинг
- **Мониторинг RAM, CPU, диска, сети** в реальном времени
- **Топ процессов** по использованию ресурсов
- **Сетевые соединения** и статистика
- **Красивые TUI интерфейсы** с анимацией

### 🌐 Сетевые инструменты
- **HTTP трассировка** запросов с детальной информацией
- **Анализ производительности** сетевых соединений

### 🐳 Docker мониторинг
- **Просмотр логов контейнеров** в реальном времени
- **TUI интерфейс** для удобного просмотра
- **Поддержка JSON логов** с парсингом

### 🗄️ Мониторинг баз данных
- **PostgreSQL и MySQL** поддержка
- **Активные соединения** и их статистика
- **Информация о таблицах** (название, размер, количество строк, индексы)
- **Медленные запросы** с деталями
- **Завершение соединений** по ID
- **Docker контейнеры** - мониторинг БД через Docker exec

## Установка

### Быстрая установка (рекомендуется)

```bash
# Установка через curl (автоматически определяет ОС и архитектуру)
curl -fsSL https://raw.githubusercontent.com/Alias1177/UNO-CLI/main/install.sh | bash
```

### Ручная установка

```bash
git clone <repository>
cd uno
go build -o uno cmd/main.go
```

### Скачивание готовых бинарных файлов

Перейдите на [GitHub Releases](https://github.com/Alias1177/UNO-CLI/releases) и скачайте подходящий файл для вашей ОС:

- **Linux**: `uno_Linux_x86_64.tar.gz` или `uno_Linux_arm64.tar.gz`
- **macOS**: `uno_Darwin_x86_64.tar.gz` или `uno_Darwin_arm64.tar.gz`
- **Windows**: `uno_Windows_x86_64.zip`

После скачивания распакуйте и добавьте в PATH:

```bash
# Linux/macOS
tar -xzf uno_Linux_x86_64.tar.gz
sudo mv uno /usr/local/bin/

# Windows
# Распакуйте zip и добавьте папку в PATH
```

## Использование

### Системный мониторинг

```bash
# Запуск мониторинга системы
./uno monitor
```

### HTTP трассировка

```bash
# Трассировка HTTP запроса
./uno trace https://example.com
```

### Docker логи

```bash
# Просмотр логов контейнера
./uno logs <container_id>
```

### Мониторинг баз данных

#### Прямое подключение к БД

```bash
# Интерактивный мониторинг БД (TUI)
./uno db monitor "postgres://user:pass@localhost:5432/dbname?sslmode=disable"
./uno db monitor "mysql://user:pass@localhost:3306/dbname"

# Просмотр таблиц
./uno db tables "postgres://user:pass@localhost:5432/dbname?sslmode=disable"

# Медленные запросы
./uno db slow-queries "postgres://user:pass@localhost:5432/dbname?sslmode=disable"

# Завершение соединения
./uno db kill "postgres://user:pass@localhost:5432/dbname?sslmode=disable" "connection_id"
```

#### Мониторинг БД в Docker контейнере

```bash
# Интерактивный мониторинг БД в контейнере (TUI)
./uno db docker monitor "container_name" "user" "password" "database"

# Просмотр таблиц в контейнере
./uno db docker tables "container_name" "user" "password" "database"

# Медленные запросы в контейнере
./uno db docker slow-queries "container_name" "user" "password" "database"

# Пример для PostgreSQL контейнера
./uno db docker monitor dp-postgres-dev dp-api string dp
./uno db docker tables dp-postgres-dev dp-api string dp
./uno db docker slow-queries dp-postgres-dev dp-api string dp

#Для просмотра логов ОШИБОК нажмите Заглавну G  и крутите стрелочками вниз и вверх
./uno logs 1635f1ecee196d0b8216d62b96ab764b002096d03f5ce9b37467e7ac63569883 -e

```

�� Управление прокруткой:
↑ / k - прокрутка вверх (на одну строку)
↓ / j - прокрутка вниз (на одну строку)
Home / g - в начало логов (самые старые)
End / G - в конец логов (самые свежие)
PageUp - на страницу вверх (быстрая прокрутка)
PageDown - на страницу вниз (быстрая прокрутка)

## Поддерживаемые базы данных

### PostgreSQL
- Автоматическое определение по connection string
- Поддержка pg_stat_statements для медленных запросов
- Информация о таблицах, индексах, размерах
- **Важно**: Добавьте `?sslmode=disable` для локальных подключений

### MySQL
- Автоматическое определение по connection string
- Поддержка Performance Schema для медленных запросов
- Статистика соединений и таблиц

## Форматы connection string

### PostgreSQL
```
postgres://username:password@host:port/database?sslmode=disable
postgresql://username:password@host:port/database?sslmode=disable
```

### MySQL
```
mysql://username:password@host:port/database
```

## TUI Управление

### Системный мониторинг
- `q`, `ctrl+c`, `esc` - выход

### Docker логи
- `q`, `ctrl+c` - выход
- `w` - переключение переноса строк

### Мониторинг БД
- `q`, `ctrl+c`, `esc` - выход
- `tab` - переключение вкладок
- `shift+tab` - переключение вкладок назад
- `k` - завершение соединения (если выбрано)

## Вкладки мониторинга БД

1. **Обзор** - общая статистика, соединения, размер БД
2. **Таблицы** - список всех таблиц с деталями
3. **Медленные запросы** - запросы с временем выполнения >1 сек

## Требования

- Go 1.24+
- Docker (для просмотра логов контейнеров и мониторинга БД в контейнерах)
- PostgreSQL/MySQL (для мониторинга БД)

## Зависимости

- `github.com/charmbracelet/bubbletea` - TUI фреймворк
- `github.com/charmbracelet/bubbles` - UI компоненты
- `github.com/spf13/cobra` - CLI фреймворк
- `github.com/shirou/gopsutil` - системная информация
- `github.com/docker/docker` - Docker API
- `github.com/lib/pq` - PostgreSQL драйвер
- `github.com/go-sql-driver/mysql` - MySQL драйвер

## Лицензия

MIT 