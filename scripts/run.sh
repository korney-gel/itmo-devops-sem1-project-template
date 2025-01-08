#!/bin/bash

set -e

echo "[run.sh] Запускаем Go-приложение..."

# Переход в корневую директорию проекта
cd "$(dirname "$0")/.."

# Запускаем сервер в фоновом режиме
go run main.go &

# Получаем PID процесса сервера
SERVER_PID=$!

# Ожидание готовности сервера
echo "[run.sh] Ожидание готовности сервера..."
until curl -s http://localhost:8080/api/v0/prices > /dev/null; do
  echo "Сервер ещё не готов, ждём 5 секунд..."
  sleep 5
done

echo "[run.sh] Сервер готов. PID: $SERVER_PID"

# Сохраняем PID процесса в файл, чтобы можно было завершить сервер позже
echo $SERVER_PID > server.pid

echo "[run.sh] Проверяем доступность сервера..."
curl -v http://localhost:8080/api/v0/prices || echo "Сервер недоступен"