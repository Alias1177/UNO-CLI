#!/bin/bash

set -e

# Определяем ОС и архитектуру
OS="$(uname -s | tr '[:upper:]' '[:lower:]')"
ARCH="$(uname -m)"

# Маппинг архитектур
case $ARCH in
    x86_64) ARCH="x86_64" ;;
    aarch64) ARCH="arm64" ;;
    arm64) ARCH="arm64" ;;
    armv7l) ARCH="arm64" ;;
    *) echo "Неподдерживаемая архитектура: $ARCH"; exit 1 ;;
esac

# Определяем формат архива
if [ "$OS" = "windows" ]; then
    EXT="zip"
else
    EXT="tar.gz"
fi

# GitHub репозиторий
REPO="Alias1177/UNO-CLI"
VERSION="latest"

# URL для скачивания
if [ "$VERSION" = "latest" ]; then
    URL="https://github.com/$REPO/releases/latest/download/UNO-CLI_${OS^}_${ARCH}.${EXT}"
else
    URL="https://github.com/$REPO/releases/download/v${VERSION}/UNO-CLI_${OS^}_${ARCH}.${EXT}"
fi

echo "Скачиваем uno для $OS/$ARCH..."
echo "URL: $URL"

# Создаем временную директорию
TMPDIR=$(mktemp -d)
cd "$TMPDIR"

# Скачиваем и распаковываем
if [ "$EXT" = "zip" ]; then
    curl -L -o uno.zip "$URL"
    unzip -o uno.zip
    rm uno.zip
else
    curl -L -o uno.tar.gz "$URL"
    tar -xzf uno.tar.gz
    rm uno.tar.gz
fi

# Перемещаем в /usr/local/bin (требует sudo)
if [ -w /usr/local/bin ]; then
    sudo mv uno /usr/local/bin/
else
    echo "Копируем в ~/.local/bin..."
    mkdir -p ~/.local/bin
    mv uno ~/.local/bin/
    echo "Добавьте ~/.local/bin в PATH:"
    echo "export PATH=\"\$HOME/.local/bin:\$PATH\""
fi

# Очищаем
cd /
rm -rf "$TMPDIR"

echo "✅ uno установлен успешно!"
echo "Используйте: uno --help" 