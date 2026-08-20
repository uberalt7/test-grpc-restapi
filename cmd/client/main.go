package main

import (
	"bytes"
	"context"
	"database/sql"
	"encoding/json"
	"fmt"
	"io"
	"log"
	"net/http"
	"time"

	pb "speedcamera/internal/gen/camera"
	"google.golang.org/grpc"
	"google.golang.org/grpc/credentials/insecure"
	_ "github.com/mattn/go-sqlite3"
)

const (
	restURL      = "http://localhost:8080/api/v1/snapshots"
	grpcURL      = "localhost:50051"
	dbPath       = "camera.db"
	iterations   = 100
	testPlate    = "BENCH001"
	testColor    = "benchmark"
	testSpeed    = 160
	testSpeedMin = 130
)

func main() {
	fmt.Println("Запуск бенчмарка REST, gRPC и прямого обращения к БД")
	fmt.Printf("Количество итераций: %d\n", iterations)
	fmt.Printf("Тестовые данные: номер %s, цвет %s, скорость %d\n", testPlate, testColor, testSpeed)
	fmt.Println()

	fmt.Println("Direct DB (SQLite)")
	dbCreateAvg := benchmarkDirectDBCreate()
	dbListAvg := benchmarkDirectDBList()

	fmt.Println()

	fmt.Println("REST API (HTTP)")
	restCreateAvg := benchmarkRESTCreate()
	restListAvg := benchmarkRESTList()

	fmt.Println()

	fmt.Println("gRPC API")
	grpcCreateAvg := benchmarkGRPCCreate()
	grpcListAvg := benchmarkGRPCList()

	fmt.Println()
	fmt.Println("Итоговые средние значения и накладные расходы (Overhead):")
	
	fmt.Printf("Direct DB Create: %v\n", dbCreateAvg)
	fmt.Printf("REST Create:      %v (Overhead: %v)\n", restCreateAvg, restCreateAvg-dbCreateAvg)
	fmt.Printf("gRPC Create:      %v (Overhead: %v)\n", grpcCreateAvg, grpcCreateAvg-dbCreateAvg)
	
	fmt.Println()
	fmt.Printf("Direct DB List:   %v\n", dbListAvg)
	fmt.Printf("REST List:        %v (Overhead: %v)\n", restListAvg, restListAvg-dbListAvg)
	fmt.Printf("gRPC List:        %v (Overhead: %v)\n", grpcListAvg, grpcListAvg-dbListAvg)
}

// ==========================================
// Direct DB (SQLite)
// ==========================================

func benchmarkDirectDBCreate() time.Duration {
	db, err := sql.Open("sqlite3", dbPath)
	if err != nil {
		log.Fatalf("Не удалось открыть БД: %v", err)
	}
	defer db.Close()

	stmt, err := db.Prepare("INSERT INTO snapshots (license_plate, color, speed, timestamp) VALUES (?, ?, ?, ?)")
	if err != nil {
		log.Fatalf("Ошибка подготовки запроса: %v", err)
	}
	defer stmt.Close()

	var total time.Duration
	for i := 0; i < iterations; i++ {
		start := time.Now()
		
		res, err := stmt.Exec(testPlate, testColor, testSpeed, time.Now().UTC())
		if err != nil {
			log.Fatalf("Ошибка вставки в БД: %v", err)
		}
		_, _ = res.LastInsertId() // Эмуляция работы репозитория
		
		total += time.Since(start)
	}
	
	avg := total / time.Duration(iterations)
	fmt.Printf("Create (среднее за %d итераций): %v\n", iterations, avg)
	return avg
}

func benchmarkDirectDBList() time.Duration {
	db, err := sql.Open("sqlite3", dbPath)
	if err != nil {
		log.Fatalf("Не удалось открыть БД: %v", err)
	}
	defer db.Close()

	var total time.Duration
	for i := 0; i < iterations; i++ {
		start := time.Now()
		
		// Запрос идентичен тому, что формирует репозиторий
		rows, err := db.Query("SELECT license_plate, color, speed, timestamp FROM snapshots WHERE color = ? AND speed >= ? ORDER BY id ASC", testColor, testSpeedMin)
		if err != nil {
			log.Fatalf("Ошибка запроса к БД: %v", err)
		}
		
		// Читаем и сканируем данные, чтобы учесть время работы с курсором БД
		for rows.Next() {
			var plate, color string
			var speed int
			var ts time.Time
			_ = rows.Scan(&plate, &color, &speed, &ts)
		}
		rows.Close()
		
		total += time.Since(start)
	}
	
	avg := total / time.Duration(iterations)
	fmt.Printf("List (среднее за %d итераций): %v\n", iterations, avg)
	return avg
}

// ==========================================
// REST (HTTP) Клиент
// ==========================================

func benchmarkRESTCreate() time.Duration {
	payload := map[string]interface{}{
		"license_plate": testPlate,
		"color":         testColor,
		"speed":         testSpeed,
	}
	bodyBytes, _ := json.Marshal(payload)

	var total time.Duration
	for i := 0; i < iterations; i++ {
		start := time.Now()
		
		req, _ := http.NewRequest(http.MethodPost, restURL, bytes.NewBuffer(bodyBytes))
		req.Header.Set("Content-Type", "application/json")
		
		resp, err := http.DefaultClient.Do(req)
		if err != nil {
			log.Fatalf("Ошибка HTTP запроса: %v", err)
		}
		
		respBody, _ := io.ReadAll(resp.Body)
		resp.Body.Close()
		
		var result map[string]interface{}
		_ = json.Unmarshal(respBody, &result)
		
		total += time.Since(start)
	}
	
	avg := total / time.Duration(iterations)
	fmt.Printf("Create (среднее за %d итераций): %v\n", iterations, avg)
	return avg
}

func benchmarkRESTList() time.Duration {
	url := fmt.Sprintf("%s?color=%s&speed_from=%d", restURL, testColor, testSpeedMin)
	var total time.Duration
	
	for i := 0; i < iterations; i++ {
		start := time.Now()
		
		resp, err := http.Get(url)
		if err != nil {
			log.Fatalf("Ошибка HTTP запроса: %v", err)
		}
		
		respBody, _ := io.ReadAll(resp.Body)
		resp.Body.Close()
		
		var result []map[string]interface{}
		_ = json.Unmarshal(respBody, &result)
		
		total += time.Since(start)
	}
	
	avg := total / time.Duration(iterations)
	fmt.Printf("List (среднее за %d итераций): %v\n", iterations, avg)
	return avg
}

// ==========================================
// gRPC Клиент
// ==========================================

func benchmarkGRPCCreate() time.Duration {
	conn, err := grpc.Dial(grpcURL, grpc.WithTransportCredentials(insecure.NewCredentials()))
	if err != nil {
		log.Fatalf("Не удалось подключиться к gRPC: %v", err)
	}
	defer conn.Close()

	client := pb.NewCameraServiceClient(conn)
	ctx := context.Background()

	var total time.Duration
	for i := 0; i < iterations; i++ {
		start := time.Now()
		
		_, err := client.CreateSnapshot(ctx, &pb.CreateSnapshotRequest{
			LicensePlate: testPlate,
			Color:        testColor,
			Speed:        int32(testSpeed),
		})
		if err != nil {
			log.Fatalf("Ошибка gRPC Create: %v", err)
		}
		
		total += time.Since(start)
	}
	
	avg := total / time.Duration(iterations)
	fmt.Printf("Create (среднее за %d итераций): %v\n", iterations, avg)
	return avg
}

func benchmarkGRPCList() time.Duration {
	conn, err := grpc.Dial(grpcURL, grpc.WithTransportCredentials(insecure.NewCredentials()))
	if err != nil {
		log.Fatalf("Не удалось подключиться к gRPC: %v", err)
	}
	defer conn.Close()

	client := pb.NewCameraServiceClient(conn)
	ctx := context.Background()

	var total time.Duration
	for i := 0; i < iterations; i++ {
		start := time.Now()
		
		_, err := client.ListSnapshots(ctx, &pb.ListSnapshotsRequest{
			Filter: &pb.SnapshotFilter{
				Color:     testColor,
				SpeedFrom: int32(testSpeedMin),
			},
		})
		if err != nil {
			log.Fatalf("Ошибка gRPC List: %v", err)
		}
		
		total += time.Since(start)
	}
	
	avg := total / time.Duration(iterations)
	fmt.Printf("List (среднее за %d итераций): %v\n", iterations, avg)
	return avg
}