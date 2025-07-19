.PHONY: build build-all clean install test

# Переменные
BINARY_NAME=uno
VERSION=$(shell git describe --tags --always --dirty)
BUILD_TIME=$(shell date -u '+%Y-%m-%d_%H:%M:%S')
LDFLAGS=-ldflags "-X main.Version=${VERSION} -X main.BuildTime=${BUILD_TIME} -X 'main.forceColors=true' -s -w"

# Сборка для текущей платформы
build:
	go build ${LDFLAGS} -o ${BINARY_NAME} ./cmd

# Сборка для всех платформ
build-all: clean
	GOOS=linux GOARCH=amd64 go build ${LDFLAGS} -o ${BINARY_NAME}_linux_amd64 ./cmd
	GOOS=linux GOARCH=arm64 go build ${LDFLAGS} -o ${BINARY_NAME}_linux_arm64 ./cmd
	GOOS=darwin GOARCH=amd64 go build ${LDFLAGS} -o ${BINARY_NAME}_darwin_amd64 ./cmd
	GOOS=darwin GOARCH=arm64 go build ${LDFLAGS} -o ${BINARY_NAME}_darwin_arm64 ./cmd
	GOOS=windows GOARCH=amd64 go build ${LDFLAGS} -o ${BINARY_NAME}_windows_amd64.exe ./cmd

# Создание архивов
package: build-all
	mkdir -p dist
	tar -czf dist/${BINARY_NAME}_linux_amd64.tar.gz ${BINARY_NAME}_linux_amd64
	tar -czf dist/${BINARY_NAME}_linux_arm64.tar.gz ${BINARY_NAME}_linux_arm64
	tar -czf dist/${BINARY_NAME}_darwin_amd64.tar.gz ${BINARY_NAME}_darwin_amd64
	tar -czf dist/${BINARY_NAME}_darwin_arm64.tar.gz ${BINARY_NAME}_darwin_arm64
	zip dist/${BINARY_NAME}_windows_amd64.zip ${BINARY_NAME}_windows_amd64.exe

# Очистка
clean:
	rm -f ${BINARY_NAME}*
	rm -rf dist/

# Установка локально
install: build
	sudo cp ${BINARY_NAME} /usr/local/bin/

# Тесты
test:
	go test ./...

# Запуск
run: build
	./${BINARY_NAME}

# Помощь
help:
	@echo "Доступные команды:"
	@echo "  build      - Сборка для текущей платформы"
	@echo "  build-all  - Сборка для всех платформ"
	@echo "  package    - Создание архивов для релиза"
	@echo "  clean      - Очистка файлов сборки"
	@echo "  install    - Установка в /usr/local/bin"
	@echo "  test       - Запуск тестов"
	@echo "  run        - Сборка и запуск"
	@echo "  help       - Показать эту справку" 