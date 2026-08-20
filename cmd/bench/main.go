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
	restURL         = "http://localhost:8080/api/v1/snapshots"
	grpcURL         = "localhost:50051"
	dbPath          = "camera.db"
	warmup          = 10
	iterations      = 1000
	listDataCount   = 100

	testPlate       = "BENCH001"
	testSpeed       = 160
	testSpeedMin    = 150

	testColorCreate = "bench_create_unique_color"
	testColorList   = "bench_list_unique_color"
)

func main() {
	fmt.Println("Запуск чистого бенчмарка REST, gRPC и прямого обращения к БД")
	fmt.Printf("Warm-up: %d, Итераций: %d\n", warmup, iterations)
	fmt.Printf("Тестовые данные: номер %s, скорость %d\n\n", testPlate, testSpeed)

	fmt.Println("Подготовка тестовых данных для List...")
	prepareListData()
	defer cleanupListData()

	// Direct DB
	fmt.Println("\nDirect DB (SQLite)")
	dbCreateAvg := benchmarkDirectDBCreate()
	cleanupCreateData()
	dbListAvg := benchmarkDirectDBList()
	fmt.Println()

	// REST API
	fmt.Println("REST API (HTTP)")
	restCreateAvg := benchmarkRESTCreate()
	cleanupCreateData()
	restListAvg := benchmarkRESTList()
	fmt.Println()

	// gRPC API
	fmt.Println("gRPC API")
	conn, err := grpc.Dial(grpcURL, grpc.WithTransportCredentials(insecure.NewCredentials()))
	if err != nil {
		log.Fatalf("Не удалось подключиться к gRPC: %v", err)
	}
	defer conn.Close()

	client := pb.NewCameraServiceClient(conn)

	grpcCreateAvg := benchmarkGRPCCreate(client)
	cleanupCreateData()
	grpcListAvg := benchmarkGRPCList(client)
	fmt.Println()

	// Итоги
	fmt.Println("Итоговые средние значения и накладные расходы (Overhead):")
	fmt.Printf("Direct DB Create: %v\n", dbCreateAvg)
	fmt.Printf("REST Create:      %v (Overhead: %v)\n", restCreateAvg, restCreateAvg-dbCreateAvg)
	fmt.Printf("gRPC Create:      %v (Overhead: %v)\n\n", grpcCreateAvg, grpcCreateAvg-dbCreateAvg)

	fmt.Printf("Direct DB List:   %v\n", dbListAvg)
	fmt.Printf("REST List:        %v (Overhead: %v)\n", restListAvg, restListAvg-dbListAvg)
	fmt.Printf("gRPC List:        %v (Overhead: %v)\n", grpcListAvg, grpcListAvg-dbListAvg)
}

// ==========================================
// Подготовка и очистка данных
// ==========================================

func prepareListData() {
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

	for i := 0; i < listDataCount; i++ {
		_, err := stmt.Exec(testPlate, testColorList, testSpeed, time.Now().UTC())
		if err != nil {
			log.Fatalf("Ошибка добавления тестовых данных для List: %v", err)
		}
	}
	fmt.Printf("Добавлено %d записей с цветом '%s' для теста List.\n", listDataCount, testColorList)
}

func cleanupListData() {
	db, err := sql.Open("sqlite3", dbPath)
	if err != nil {
		return
	}
	defer db.Close()
	db.Exec("DELETE FROM snapshots WHERE color = ?", testColorList)
	fmt.Println("Тестовые данные для List удалены.")
}

func cleanupCreateData() {
	db, err := sql.Open("sqlite3", dbPath)
	if err != nil {
		return
	}
	defer db.Close()
	db.Exec("DELETE FROM snapshots WHERE color = ?", testColorCreate)
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

	for i := 0; i < warmup; i++ {
		stmt.Exec(testPlate, testColorCreate, testSpeed, time.Now().UTC())
	}

	var total time.Duration
	for i := 0; i < iterations; i++ {
		start := time.Now()
		res, _ := stmt.Exec(testPlate, testColorCreate, testSpeed, time.Now().UTC())
		_, _ = res.LastInsertId()
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

	for i := 0; i < warmup; i++ {
		rows, _ := db.Query("SELECT license_plate, color, speed, timestamp FROM snapshots WHERE color = ? AND speed >= ? ORDER BY id ASC", testColorList, testSpeedMin)
		for rows.Next() {}
		rows.Close()
	}

	var total time.Duration
	for i := 0; i < iterations; i++ {
		start := time.Now()
		rows, _ := db.Query("SELECT license_plate, color, speed, timestamp FROM snapshots WHERE color = ? AND speed >= ? ORDER BY id ASC", testColorList, testSpeedMin)
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
// REST (HTTP) Бенчмарк
// ==========================================

func benchmarkRESTCreate() time.Duration {
	payload := map[string]interface{}{
		"license_plate": testPlate,
		"color":         testColorCreate,
		"speed":         testSpeed,
	}
	bodyBytes, _ := json.Marshal(payload)

	for i := 0; i < warmup; i++ {
		req, _ := http.NewRequest(http.MethodPost, restURL, bytes.NewBuffer(bodyBytes))
		req.Header.Set("Content-Type", "application/json")
		resp, _ := http.DefaultClient.Do(req)
		io.ReadAll(resp.Body)
		resp.Body.Close()
	}

	var total time.Duration
	for i := 0; i < iterations; i++ {
		start := time.Now()
		req, _ := http.NewRequest(http.MethodPost, restURL, bytes.NewBuffer(bodyBytes))
		req.Header.Set("Content-Type", "application/json")
		resp, _ := http.DefaultClient.Do(req)
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
	url := fmt.Sprintf("%s?color=%s&speed_from=%d", restURL, testColorList, testSpeedMin)

	for i := 0; i < warmup; i++ {
		resp, _ := http.Get(url)
		io.ReadAll(resp.Body)
		resp.Body.Close()
	}

	var total time.Duration
	for i := 0; i < iterations; i++ {
		start := time.Now()
		resp, _ := http.Get(url)
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
// gRPC Бенчмарк
// ==========================================

func benchmarkGRPCCreate(client pb.CameraServiceClient) time.Duration {
	ctx := context.Background()

	for i := 0; i < warmup; i++ {
		client.CreateSnapshot(ctx, &pb.CreateSnapshotRequest{
			LicensePlate: testPlate, Color: testColorCreate, Speed: int32(testSpeed),
		})
	}

	var total time.Duration
	for i := 0; i < iterations; i++ {
		start := time.Now()
		_, _ = client.CreateSnapshot(ctx, &pb.CreateSnapshotRequest{
			LicensePlate: testPlate,
			Color:        testColorCreate,
			Speed:        int32(testSpeed),
		})
		total += time.Since(start)
	}

	avg := total / time.Duration(iterations)
	fmt.Printf("Create (среднее за %d итераций): %v\n", iterations, avg)
	return avg
}

func benchmarkGRPCList(client pb.CameraServiceClient) time.Duration {
	ctx := context.Background()
	req := &pb.ListSnapshotsRequest{
		Filter: &pb.SnapshotFilter{
			Color:     testColorList,
			SpeedFrom: int32(testSpeedMin),
		},
	}

	for i := 0; i < warmup; i++ {
		client.ListSnapshots(ctx, req)
	}

	var total time.Duration
	for i := 0; i < iterations; i++ {
		start := time.Now()
		_, _ = client.ListSnapshots(ctx, req)
		total += time.Since(start)
	}

	avg := total / time.Duration(iterations)
	fmt.Printf("List (среднее за %d итераций): %v\n", iterations, avg)
	return avg
}