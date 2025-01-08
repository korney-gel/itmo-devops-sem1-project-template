#!/bin/bash

set -e

echo "[run.sh] Запускаем Go-приложение..."

# Переход в корневую директорию проекта
cd "$(dirname "$0")/.."

# Проверка подключения к базе данных
echo "[run.sh] Проверяем подключение к базе данных..."
PGPASSWORD=$POSTGRES_PASSWORD psql -h $POSTGRES_HOST -U $POSTGRES_USER -d $POSTGRES_DB -c '\q'
if [ $? -eq 0 ]; then
  echo "[run.sh] Подключение к базе данных успешно."
else
  echo "[run.sh] Ошибка подключения к базе данных."
  exit 1
fi

echo "[run.sh] Проверяем список таблиц в базе данных...."
PGPASSWORD=$POSTGRES_PASSWORD psql -h $POSTGRES_HOST -U $POSTGRES_USER -d $POSTGRES_DB -c '\dt *.*'

# Проверка структуры таблицы prices
echo "[run.sh] Проверяем структуру таблицы prices..."
PGPASSWORD=$POSTGRES_PASSWORD psql -h $POSTGRES_HOST -U $POSTGRES_USER -d $POSTGRES_DB -c '\d prices'
if [ $? -eq 0 ]; then
  echo "[run.sh] Таблица prices существует."
else
  echo "[run.sh] Таблица prices отсутствует или её структура некорректна."
  exit 1
fi

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