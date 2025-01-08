package main

import (
	"archive/zip"
	"bytes"
	"context"
	"database/sql"
	"encoding/csv"
	"encoding/json"
	"fmt"
	_ "github.com/lib/pq" // драйвер PostgreSQL
	"io"
	"log"
	"net/http"
	"strconv"
	"strings"
)

const (
	dbHost     = "127.0.0.1"
	dbPort     = "5432"
	dbUser     = "validator"
	dbPassword = "val1dat0r"
	dbName     = "project-sem-1"
)

// Структура для возврата JSON из POST
type ImportResponse struct {
	TotalItems      int `json:"total_items"`
	TotalCategories int `json:"total_categories"`
	TotalPrice      int `json:"total_price"`
}

func main() {
	// Подключаемся к БД
	log.Printf("Подключаемся к базе данных: хост=%s, порт=%s, имя БД=%s, пользователь=%s",
		dbHost, dbPort, dbName, dbUser)

	connStr := fmt.Sprintf("host=%s port=%s user=%s password=%s dbname=%s sslmode=disable",
		dbHost, dbPort, dbUser, dbPassword, dbName)
	db, err := sql.Open("postgres", connStr)
	if err != nil {
		log.Fatalf("Ошибка подключения к БД: %v", err)
	}
	defer db.Close()

	// Проверяем соединение
	if err := db.Ping(); err != nil {
		log.Fatalf("БД не отвечает: %v", err)
	}

	log.Println("Подключение к БД успешно")

	// Роуты
	http.HandleFunc("/api/v0/prices", func(w http.ResponseWriter, r *http.Request) {
		log.Printf("Запрос на /api/v0/prices: метод=%s", r.Method)
		switch r.Method {
		case http.MethodPost:
			handlePostPrices(w, r, db)
		case http.MethodGet:
			handleGetPrices(w, r, db)
		default:
			log.Printf("Метод не поддерживается: %s", r.Method)
			http.Error(w, "Method not allowed", http.StatusMethodNotAllowed)
		}
	})

	// Запуск сервера
	fmt.Println("Сервер слушает порт 8080...")
	log.Println("Сервер слушает на порту 8080...")
	log.Fatal(http.ListenAndServe(":8080", nil))
}

// Обработчик POST /api/v0/prices — загружает zip, парсит CSV, сохраняет в БД
func handlePostPrices(w http.ResponseWriter, r *http.Request, db *sql.DB) {
	// Принимаем zip-архив из тела запроса
	if err := r.ParseMultipartForm(10 << 20); err != nil {
		http.Error(w, "Ошибка парсинга запроса", http.StatusBadRequest)
		return
	}

	// Предположим, что в теле будет одно поле с именем "file" (можно расширять под нужды)
	file, _, err := r.FormFile("file")
	if err != nil {
		http.Error(w, "Не удалось прочитать файл из запроса", http.StatusBadRequest)
		return
	}
	defer file.Close()

	// Считываем всё в память (для простоты). Если файл большой, лучше потоково.
	var buf bytes.Buffer
	if _, err := io.Copy(&buf, file); err != nil {
		http.Error(w, "Ошибка копирования файла", http.StatusInternalServerError)
		return
	}

	// Распаковываем zip
	zipReader, err := zip.NewReader(bytes.NewReader(buf.Bytes()), int64(buf.Len()))
	if err != nil {
		http.Error(w, "Ошибка чтения zip-архива", http.StatusBadRequest)
		return
	}

	var totalItems int
	var totalPrice float64
	categoriesMap := make(map[string]bool)

	// Ищем data.csv в архиве
	for _, zipFile := range zipReader.File {
		if strings.HasSuffix(zipFile.Name, "data.csv") {
			log.Printf("Обрабатываю файл: %s", zipFile.Name)
			f, err := zipFile.Open()
			if err != nil {
				http.Error(w, "Ошибка открытия data.csv внутри zip", http.StatusInternalServerError)
				return
			}
			defer f.Close()

			csvReader := csv.NewReader(f)
			headers, err := csvReader.Read() // Читаем заголовок
			if err != nil {
				http.Error(w, "Ошибка чтения заголовка CSV", http.StatusBadRequest)
				return
			}

			log.Printf("Заголовки CSV: %v", headers)

			for {
				row, err := csvReader.Read()
				if err == io.EOF {
					break
				}
				if err != nil {
					http.Error(w, "Ошибка чтения CSV", http.StatusInternalServerError)
					return
				}

				if len(row) < 5 {
					log.Printf("Пропущена строка: %v", row)
					continue
				}

				productID := row[0]
				productName := row[1]
				category := row[2]
				price := row[3]
				createdAt := row[4]

				// Парсинг цены
				priceFloat, err := strconv.ParseFloat(price, 64)
				if err != nil {
					http.Error(w, "Некорректный формат цены: "+price, http.StatusBadRequest)
					return
				}

				// Запись в БД
				log.Printf("Вставка в БД: product_id=%s, created_at=%s, product_name=%s, category=%s, price=%f",
					productID, createdAt, productName, category, priceFloat)

				_, err = db.ExecContext(context.Background(),
					`INSERT INTO prices (product_id, created_at, product_name, category, price)
                     VALUES ($1, $2, $3, $4, $5)`,
					productID, createdAt, productName, category, priceFloat)
				if err != nil {
					log.Printf("Ошибка записи в БД: %v", err)
					http.Error(w, "Ошибка записи в БД", http.StatusInternalServerError)
					return
				}

				totalItems++
				totalPrice += priceFloat
				categoriesMap[category] = true
			}
		}
	}

	// Формируем ответ
	resp := ImportResponse{
		TotalItems:      totalItems,
		TotalCategories: len(categoriesMap),
		TotalPrice:      int(totalPrice), // Преобразуем в int для ответа
	}

	w.Header().Set("Content-Type", "application/json")
	json.NewEncoder(w).Encode(resp)
}

// Обработчик GET /api/v0/prices — выгружает все данные из БД, формирует data.csv и отдаёт в zip
func handleGetPrices(w http.ResponseWriter, r *http.Request, db *sql.DB) {
	rows, err := db.QueryContext(context.Background(), "SELECT product_id, created_at, product_name, category, price FROM prices")
	if err != nil {
		http.Error(w, "Ошибка запроса к БД", http.StatusInternalServerError)
		return
	}
	defer rows.Close()

	var csvBuf bytes.Buffer
	csvWriter := csv.NewWriter(&csvBuf)

	for rows.Next() {
		var productID, createdAt, productName, category string
		var price string // или использовать decimal/float
		if err := rows.Scan(&productID, &createdAt, &productName, &category, &price); err != nil {
			http.Error(w, "Ошибка чтения из БД", http.StatusInternalServerError)
			return
		}

		record := []string{productID, createdAt, productName, category, price}
		if err := csvWriter.Write(record); err != nil {
			http.Error(w, "Ошибка записи строки CSV", http.StatusInternalServerError)
			return
		}
	}
	csvWriter.Flush()

	// Формируем zip-архив в памяти
	var zipBuf bytes.Buffer
	zipWriter := zip.NewWriter(&zipBuf)
	fileInZip, err := zipWriter.Create("data.csv")
	if err != nil {
		http.Error(w, "Ошибка создания файла в zip", http.StatusInternalServerError)
		return
	}

	if _, err := fileInZip.Write(csvBuf.Bytes()); err != nil {
		http.Error(w, "Ошибка записи в zip-архив", http.StatusInternalServerError)
		return
	}
	zipWriter.Close()

	// Отдаём готовый zip
	w.Header().Set("Content-Disposition", "attachment; filename=\"prices.zip\"")
	w.Header().Set("Content-Type", "application/zip")
	w.WriteHeader(http.StatusOK)
	_, _ = w.Write(zipBuf.Bytes())
}
