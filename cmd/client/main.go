package main

import (
	"bytes"
	"context"
	"encoding/json"
	"fmt"
	"io"
	"log"
	"net/http"
	"time"

	pb "speedcamera/internal/gen/camera"
	"google.golang.org/grpc"
	"google.golang.org/grpc/credentials/insecure"
)

const (
	restURL = "http://localhost:8080/api/v1/snapshots"
	grpcURL = "localhost:50051"
)

func main() {
	fmt.Println("Запуск тестовых клиентов REST и gRPC")
	fmt.Println()

	restPassed, restTotal := runRESTTests()
	fmt.Println()

	grpcPassed, grpcTotal := runGRPCTests()
	fmt.Println()

	fmt.Println("Итоговые результаты:")
	fmt.Printf("REST: %d/%d тестов пройдено\n", restPassed, restTotal)
	fmt.Printf("gRPC: %d/%d тестов пройдено\n", grpcPassed, grpcTotal)
	fmt.Println()

	if restPassed == restTotal && grpcPassed == grpcTotal {
		fmt.Println("ВСЕ ТЕСТЫ ПРОЙДЕНЫ УСПЕШНО")
	} else {
		fmt.Println("ЕСТЬ ПРОВАЛЕННЫЕ ТЕСТЫ")
	}
}

// ==========================================
// REST Тесты
// ==========================================

func runRESTTests() (int, int) {
	fmt.Println("REST API (HTTP)")
	fmt.Println("----------------------------------------")

	passed := 0
	total := 0

	// Тест 1: Создание снимка
	total++
	if testRESTCreate() {
		passed++
	}

	// Тест 2: Получение всех записей
	total++
	if testRESTListAll() {
		passed++
	}

	// Тест 3: Фильтр по цвету
	total++
	if testRESTListFilterColor() {
		passed++
	}

	// Тест 4: Проверка, что ID не возвращается в List
	total++
	if testRESTListNoID() {
		passed++
	}

	// Тест 5: Валидация номера (длиннее 10 символов)
	total++
	if testRESTCreateValidation() {
		passed++
	}

	return passed, total
}

func testRESTCreate() bool {
	payload := map[string]interface{}{
		"license_plate": "TEST001",
		"color":         "test_rest_color",
		"speed":         120,
	}
	bodyBytes, _ := json.Marshal(payload)

	req, _ := http.NewRequest(http.MethodPost, restURL, bytes.NewBuffer(bodyBytes))
	req.Header.Set("Content-Type", "application/json")

	resp, err := http.DefaultClient.Do(req)
	if err != nil {
		fmt.Printf("  [FAIL] Create: ошибка запроса: %v\n", err)
		return false
	}
	defer resp.Body.Close()

	if resp.StatusCode != http.StatusCreated {
		fmt.Printf("  [FAIL] Create: ожидаемый статус 201, получен %d\n", resp.StatusCode)
		return false
	}

	respBody, _ := io.ReadAll(resp.Body)
	var result map[string]interface{}
	if err := json.Unmarshal(respBody, &result); err != nil {
		fmt.Printf("  [FAIL] Create: ошибка парсинга ответа: %v\n", err)
		return false
	}

	id, okID := result["id"]
	ts, okTS := result["timestamp"]
	if !okID || !okTS {
		fmt.Printf("  [FAIL] Create: в ответе отсутствуют поля id или timestamp\n")
		return false
	}

	fmt.Printf("  [PASS] Create: создан снимок с id=%v, timestamp=%v\n", id, ts)
	return true
}

func testRESTListAll() bool {
	resp, err := http.Get(restURL)
	if err != nil {
		fmt.Printf("  [FAIL] List All: ошибка запроса: %v\n", err)
		return false
	}
	defer resp.Body.Close()

	if resp.StatusCode != http.StatusOK {
		fmt.Printf("  [FAIL] List All: ожидаемый статус 200, получен %d\n", resp.StatusCode)
		return false
	}

	respBody, _ := io.ReadAll(resp.Body)
	var result []map[string]interface{}
	if err := json.Unmarshal(respBody, &result); err != nil {
		fmt.Printf("  [FAIL] List All: ошибка парсинга ответа: %v\n", err)
		return false
	}

	if len(result) == 0 {
		fmt.Printf("  [FAIL] List All: получен пустой список\n")
		return false
	}

	fmt.Printf("  [PASS] List All: получено %d записей\n", len(result))
	return true
}

func testRESTListFilterColor() bool {
	// Сначала создаем запись с уникальным цветом
	uniqueColor := fmt.Sprintf("test_color_%d", time.Now().UnixNano())
	payload := map[string]interface{}{
		"license_plate": "FLT001",
		"color":         uniqueColor,
		"speed":         100,
	}
	bodyBytes, _ := json.Marshal(payload)
	req, _ := http.NewRequest(http.MethodPost, restURL, bytes.NewBuffer(bodyBytes))
	req.Header.Set("Content-Type", "application/json")
	resp, _ := http.DefaultClient.Do(req)
	io.ReadAll(resp.Body)
	resp.Body.Close()

	// Теперь запрашиваем по фильтру
	url := fmt.Sprintf("%s?color=%s", restURL, uniqueColor)
	resp, err := http.Get(url)
	if err != nil {
		fmt.Printf("  [FAIL] List Filter: ошибка запроса: %v\n", err)
		return false
	}
	defer resp.Body.Close()

	respBody, _ := io.ReadAll(resp.Body)
	var result []map[string]interface{}
	if err := json.Unmarshal(respBody, &result); err != nil {
		fmt.Printf("  [FAIL] List Filter: ошибка парсинга ответа: %v\n", err)
		return false
	}

	if len(result) == 0 {
		fmt.Printf("  [FAIL] List Filter: по фильтру color=%s ничего не найдено\n", uniqueColor)
		return false
	}

	for _, item := range result {
		if item["color"] != uniqueColor {
			fmt.Printf("  [FAIL] List Filter: найдена запись с неверным цветом: %v\n", item["color"])
			return false
		}
	}

	fmt.Printf("  [PASS] List Filter: по фильтру color=%s найдено %d записей, все с корректным цветом\n", uniqueColor, len(result))
	return true
}

func testRESTListNoID() bool {
	resp, err := http.Get(restURL)
	if err != nil {
		fmt.Printf("  [FAIL] List No ID: ошибка запроса: %v\n", err)
		return false
	}
	defer resp.Body.Close()

	respBody, _ := io.ReadAll(resp.Body)
	var result []map[string]interface{}
	if err := json.Unmarshal(respBody, &result); err != nil {
		fmt.Printf("  [FAIL] List No ID: ошибка парсинга ответа: %v\n", err)
		return false
	}

	for _, item := range result {
		if _, exists := item["id"]; exists {
			fmt.Printf("  [FAIL] List No ID: в ответе присутствует поле id\n")
			return false
		}
	}

	fmt.Printf("  [PASS] List No ID: поле id отсутствует во всех %d записях\n", len(result))
	return true
}

func testRESTCreateValidation() bool {
	payload := map[string]interface{}{
		"license_plate": "TOOLONGPLATE12345", // длиннее 10 символов
		"color":         "test",
		"speed":         100,
	}
	bodyBytes, _ := json.Marshal(payload)

	req, _ := http.NewRequest(http.MethodPost, restURL, bytes.NewBuffer(bodyBytes))
	req.Header.Set("Content-Type", "application/json")

	resp, err := http.DefaultClient.Do(req)
	if err != nil {
		fmt.Printf("  [FAIL] Create Validation: ошибка запроса: %v\n", err)
		return false
	}
	defer resp.Body.Close()
	io.ReadAll(resp.Body)

	if resp.StatusCode == http.StatusCreated {
		fmt.Printf("  [FAIL] Create Validation: сервер принял номер длиннее 10 символов\n")
		return false
	}

	fmt.Printf("  [PASS] Create Validation: сервер отклонил номер длиннее 10 символов (статус %d)\n", resp.StatusCode)
	return true
}

// ==========================================
// gRPC Тесты
// ==========================================

func runGRPCTests() (int, int) {
	fmt.Println("gRPC API")
	fmt.Println("----------------------------------------")

	conn, err := grpc.Dial(grpcURL, grpc.WithTransportCredentials(insecure.NewCredentials()))
	if err != nil {
		log.Fatalf("Не удалось подключиться к gRPC: %v", err)
	}
	defer conn.Close()

	client := pb.NewCameraServiceClient(conn)
	ctx := context.Background()

	passed := 0
	total := 0

	// Тест 1: Создание снимка
	total++
	if testGRPCCreate(ctx, client) {
		passed++
	}

	// Тест 2: Получение всех записей
	total++
	if testGRPCListAll(ctx, client) {
		passed++
	}

	// Тест 3: Фильтр по цвету
	total++
	if testGRPCListFilterColor(ctx, client) {
		passed++
	}

	// Тест 4: Валидация номера (длиннее 10 символов)
	total++
	if testGRPCCreateValidation(ctx, client) {
		passed++
	}

	return passed, total
}

func testGRPCCreate(ctx context.Context, client pb.CameraServiceClient) bool {
	resp, err := client.CreateSnapshot(ctx, &pb.CreateSnapshotRequest{
		LicensePlate: "TEST002",
		Color:        "test_grpc_color",
		Speed:        130,
	})
	if err != nil {
		fmt.Printf("  [FAIL] Create: ошибка запроса: %v\n", err)
		return false
	}

	if resp.Id == 0 {
		fmt.Printf("  [FAIL] Create: получен id=0\n")
		return false
	}

	fmt.Printf("  [PASS] Create: создан снимок с id=%d, timestamp=%v\n", resp.Id, resp.Timestamp.AsTime())
	return true
}

func testGRPCListAll(ctx context.Context, client pb.CameraServiceClient) bool {
	resp, err := client.ListSnapshots(ctx, &pb.ListSnapshotsRequest{})
	if err != nil {
		fmt.Printf("  [FAIL] List All: ошибка запроса: %v\n", err)
		return false
	}

	if len(resp.Snapshots) == 0 {
		fmt.Printf("  [FAIL] List All: получен пустой список\n")
		return false
	}

	fmt.Printf("  [PASS] List All: получено %d записей\n", len(resp.Snapshots))
	return true
}

func testGRPCListFilterColor(ctx context.Context, client pb.CameraServiceClient) bool {
	uniqueColor := fmt.Sprintf("test_grpc_color_%d", time.Now().UnixNano())

	// Создаем запись с уникальным цветом
	_, err := client.CreateSnapshot(ctx, &pb.CreateSnapshotRequest{
		LicensePlate: "FLT002",
		Color:        uniqueColor,
		Speed:        110,
	})
	if err != nil {
		fmt.Printf("  [FAIL] List Filter: ошибка создания записи: %v\n", err)
		return false
	}

	// Запрашиваем по фильтру
	resp, err := client.ListSnapshots(ctx, &pb.ListSnapshotsRequest{
		Filter: &pb.SnapshotFilter{
			Color: uniqueColor,
		},
	})
	if err != nil {
		fmt.Printf("  [FAIL] List Filter: ошибка запроса: %v\n", err)
		return false
	}

	if len(resp.Snapshots) == 0 {
		fmt.Printf("  [FAIL] List Filter: по фильтру color=%s ничего не найдено\n", uniqueColor)
		return false
	}

	for _, snap := range resp.Snapshots {
		if snap.Color != uniqueColor {
			fmt.Printf("  [FAIL] List Filter: найдена запись с неверным цветом: %s\n", snap.Color)
			return false
		}
	}

	fmt.Printf("  [PASS] List Filter: по фильтру color=%s найдено %d записей, все с корректным цветом\n", uniqueColor, len(resp.Snapshots))
	return true
}

func testGRPCCreateValidation(ctx context.Context, client pb.CameraServiceClient) bool {
	_, err := client.CreateSnapshot(ctx, &pb.CreateSnapshotRequest{
		LicensePlate: "TOOLONGPLATE12345", // длиннее 10 символов
		Color:        "test",
		Speed:        100,
	})

	if err == nil {
		fmt.Printf("  [FAIL] Create Validation: сервер принял номер длиннее 10 символов\n")
		return false
	}

	fmt.Printf("  [PASS] Create Validation: сервер отклонил номер длиннее 10 символов: %v\n", err)
	return true
}