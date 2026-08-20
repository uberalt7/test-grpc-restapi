package domain

import (
	"context"
	"errors"
	"time"
)

// Snapshot представляет запись о фиксации скорости.
// ID не включен в базовую модель для чтения, так как он не возвращается в List.
type Snapshot struct {
	ID           int64     // Используется внутри для сортировки и в ответе Create
	LicensePlate string
	Color        string
	Speed        int
	Timestamp    time.Time
}

type CreateSnapshotInput struct {
	LicensePlate string
	Color        string
	Speed        int
}

type FilterSnapshotsInput struct {
	TimeFrom  *time.Time
	TimeTo    *time.Time
	Color     *string
	SpeedFrom *int
	SpeedTo   *int
}

// Repository описывает контракт для работы с БД.
type Repository interface {
	CreateSnapshot(ctx context.Context, input CreateSnapshotInput) (Snapshot, error)
	ListSnapshots(ctx context.Context, filter FilterSnapshotsInput) ([]Snapshot, error)
}

var (
	ErrLicensePlateTooLong = errors.New("license plate must be 10 characters or less")
)