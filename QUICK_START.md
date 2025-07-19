# Быстрый старт

## Установка

```bash
# Автоматическая установка (Linux/macOS)
curl -fsSL https://raw.githubusercontent.com/Alias1177/UNO-CLI/main/install.sh | bash

# Или скачайте вручную с GitHub Releases
```

## Использование

```bash
# Системный мониторинг
uno monitor

# Docker логи
uno logs <container_id>

# Мониторинг PostgreSQL
uno db monitor "postgres://user:pass@localhost:5432/dbname?sslmode=disable"

# Мониторинг MySQL
uno db monitor "mysql://user:pass@localhost:3306/dbname"

# HTTP трассировка
uno trace https://example.com
```

## Поддерживаемые платформы

- ✅ Linux (x86_64, ARM64)
- ✅ macOS (Intel, Apple Silicon)
- ✅ Windows (x86_64)

## Что дальше?

1. Создайте GitHub репозиторий
2. Замените `YOUR_USERNAME` в файлах на ваше имя пользователя
3. Создайте тег `v1.0.0` для первого релиза
4. GitHub Actions автоматически создаст мультиплатформенные бинарные файлы

Пользователи смогут установить вашу CLI одной командой! 