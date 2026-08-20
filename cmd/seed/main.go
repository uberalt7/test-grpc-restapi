package main

import (
	"database/sql"
	"fmt"
	"log"
	"math/rand"
	"time"

	_ "github.com/mattn/go-sqlite3"
)

func main() {
	// Удаляем старую базу, если она есть, чтобы начать с чистого листа
	// (Опционально, можно закомментировать, если хотите добавлять к существующим)
	
	db, err := sql.Open("sqlite3", "camera.db")
	if err != nil {
		log.Fatalf("Failed to open DB: %v", err)
	}
	defer db.Close()

	// Создаем таблицу (точно такую же, как в основном проекте)
	createTableSQL := `CREATE TABLE IF NOT EXISTS snapshots (
		id INTEGER PRIMARY KEY AUTOINCREMENT,
		license_plate TEXT NOT NULL,
		color TEXT NOT NULL,
		speed INTEGER NOT NULL,
		timestamp DATETIME NOT NULL
	);`
	
	if _, err := db.Exec(createTableSQL); err != nil {
		log.Fatalf("Failed to create table: %v", err)
	}

	// Очищаем таблицу перед заполнением
	db.Exec("DELETE FROM snapshots")

	// Наборы данных для генерации
	colors := []string{"red", "blue", "black", "white", "gray", "green", "yellow"}
	letters := []rune("ABEKMHOPCTYX")
	regions := []string{"42", "142", "54", "154", "77", "99", "50", "01", "102", "777", "67", "76", "123"}

	// Инициализация генератора случайных чисел
	r := rand.New(rand.NewSource(time.Now().UnixNano()))

	now := time.Now().UTC()
	thirtyDaysAgo := now.AddDate(0, 0, -30)

	stmt, err := db.Prepare("INSERT INTO snapshots (license_plate, color, speed, timestamp) VALUES (?, ?, ?, ?)")
	if err != nil {
		log.Fatalf("Failed to prepare statement: %v", err)
	}
	defer stmt.Close()

	totalRecords := 100_000
	fmt.Println("Начинается добавление", totalRecords, "товаров!")

	for i := 0; i < totalRecords; i++ {
		// 1. Генерация номера (например, А123БВ77)
		l1 := letters[r.Intn(len(letters))]
		l2 := letters[r.Intn(len(letters))]
		l3 := letters[r.Intn(len(letters))]
		nums := r.Intn(900) + 100 // от 100 до 999
		region := regions[r.Intn(len(regions))]
		plate := fmt.Sprintf("%c%03d%c%c%s", l1, nums, l2, l3, region)

		// 2. Генерация цвета
		color := colors[r.Intn(len(colors))]

		// 3. Генерация скорости (от 40 до 220 км/ч)
		speed := r.Intn(180) + 40 

		// 4. Генерация времени (случайное время за последние 30 дней)
		randomSeconds := r.Int63n(int64(now.Sub(thirtyDaysAgo).Seconds()))
		ts := thirtyDaysAgo.Add(time.Duration(randomSeconds) * time.Second)

		// Вставляем в БД (время в формате RFC3339, чтобы REST API мог его легко парсить)
		_, err = stmt.Exec(plate, color, speed, ts.Format(time.RFC3339))
		if err != nil {
			log.Fatalf("Failed to insert record: %v", err)
		}
		if i % 5000 == 4999 {
			fmt.Println("Добавлено", i+1, "товаров из", totalRecords)
		}
	}

	fmt.Printf("База camera.db успешно создана и заполнена %d записями!\n", totalRecords)
	fmt.Println("Данные распределены за последние 30 дней. Скорость от 40 до 220.")
}