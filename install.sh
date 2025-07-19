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
    URL="https://github.com/$REPO/releases/latest/download/UNO-CLI_Darwin_${ARCH}.${EXT}"
else
    URL="https://github.com/$REPO/releases/download/v${VERSION}/UNO-CLI_Darwin_${ARCH}.${EXT}"
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
    echo "✅ uno установлен в /usr/local/bin"
else
    echo "Копируем в ~/.local/bin..."
    mkdir -p ~/.local/bin
    mv uno ~/.local/bin/
    
    # Добавляем в PATH автоматически
    SHELL_CONFIG=""
    if [ -f "$HOME/.zshrc" ]; then
        SHELL_CONFIG="$HOME/.zshrc"
    elif [ -f "$HOME/.bashrc" ]; then
        SHELL_CONFIG="$HOME/.bashrc"
    elif [ -f "$HOME/.bash_profile" ]; then
        SHELL_CONFIG="$HOME/.bash_profile"
    fi
    
    if [ -n "$SHELL_CONFIG" ]; then
        # Проверяем что PATH еще не добавлен
        if ! grep -q "~/.local/bin" "$SHELL_CONFIG"; then
            echo "" >> "$SHELL_CONFIG"
            echo "# UNO CLI PATH" >> "$SHELL_CONFIG"
            echo 'export PATH="$HOME/.local/bin:$PATH"' >> "$SHELL_CONFIG"
            echo "✅ PATH добавлен в $SHELL_CONFIG"
        fi
        
        # Обновляем текущую сессию
        export PATH="$HOME/.local/bin:$PATH"
        echo "✅ PATH обновлен для текущей сессии"
    else
        echo "⚠️  Добавьте вручную в PATH:"
        echo "export PATH=\"\$HOME/.local/bin:\$PATH\""
    fi
fi

# Очищаем
cd /
rm -rf "$TMPDIR"

echo "✅ uno установлен успешно!"
echo "Используйте: uno --help" 