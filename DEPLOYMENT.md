# Инструкция по деплою

## Настройка GitHub репозитория

### 1. Создайте GitHub репозиторий

```bash
git init
git add .
git commit -m "Initial commit"
git branch -M main
git remote add origin git@github.com:Alias1177/UNO-CLI.git
git push -u origin main
```

### 2. Обновите ссылки в файлах

Ссылки уже обновлены для репозитория Alias1177/UNO-CLI

### 3. Создайте первый релиз

```bash
# Создайте тег
git tag v1.0.0
git push origin v1.0.0
```

GitHub Actions автоматически создаст релиз с бинарными файлами для всех платформ.

## Локальная сборка для тестирования

```bash
# Сборка для текущей платформы
make build

# Сборка для всех платформ
make build-all

# Создание архивов
make package
```

## Проверка установки

После создания релиза пользователи смогут установить CLI:

```bash
# Автоматическая установка
curl -fsSL https://raw.githubusercontent.com/Alias1177/UNO-CLI/main/install.sh | bash

# Или скачать вручную с GitHub Releases
```

## Структура релиза

Каждый релиз будет содержать:
- `uno_Linux_x86_64.tar.gz` - Linux AMD64
- `uno_Linux_arm64.tar.gz` - Linux ARM64
- `uno_Darwin_x86_64.tar.gz` - macOS Intel
- `uno_Darwin_arm64.tar.gz` - macOS Apple Silicon
- `uno_Windows_x86_64.zip` - Windows AMD64
- `checksums.txt` - контрольные суммы

## Обновление версии

```bash
# Создайте новый тег
git tag v1.1.0
git push origin v1.1.0
```

## Troubleshooting

### Проблемы с GitHub Actions
- Убедитесь что в репозитории включены GitHub Actions
- Проверьте что у workflow есть права на создание релизов

### Проблемы с установкой
- Проверьте что скрипт `install.sh` доступен по ссылке
- Убедитесь что бинарные файлы загружены в релиз

### Локальная сборка
```bash
# Если нет git тегов
git tag v0.0.1
make build
``` 