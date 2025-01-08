#!/bin/bash
#!/usr/bin/env bash
set -e

echo "[prepare.sh] Устанавливаем зависимости Go..."
go mod tidy

echo "[prepare.sh] Проверяем, что контейнер с БД запущен..."
docker compose up -d


echo "[prepare.sh] Ожидание готовности базы данных..."
until docker compose exec db pg_isready -U validator -d project-sem-1; do
  echo "База данных ещё не готова, ждём 5 секунд..."
  sleep 5
done
echo "База данных готова."

echo "[prepare.sh] Создаём таблицу prices (если не существует)..."
docker compose exec db \
  psql -U validator -d project-sem-1 -c \
  "CREATE TABLE IF NOT EXISTS public.prices (
    product_id TEXT,
    created_at DATE,
    product_name TEXT,
    category TEXT,
    price NUMERIC
);"

echo "[prepare.sh] Скрипт подготовки завершён."