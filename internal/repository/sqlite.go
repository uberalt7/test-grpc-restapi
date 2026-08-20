package repository

import (
	"context"
	"database/sql"
	"fmt"
	"time"

	"speedcamera/internal/domain"
	_ "github.com/mattn/go-sqlite3"
)

type SQLiteRepo struct {
	db *sql.DB
}

func NewSQLiteRepo(dbPath string) (*SQLiteRepo, error) {
	db, err := sql.Open("sqlite3", dbPath)
	if err != nil {
		return nil, err
	}

	// Создание таблицы
	createTableSQL := `CREATE TABLE IF NOT EXISTS snapshots (
		id INTEGER PRIMARY KEY AUTOINCREMENT,
		license_plate TEXT NOT NULL,
		color TEXT NOT NULL,
		speed INTEGER NOT NULL,
		timestamp DATETIME NOT NULL
	);`
	
	if _, err := db.Exec(createTableSQL); err != nil {
		return nil, fmt.Errorf("failed to create table: %w", err)
	}

	return &SQLiteRepo{db: db}, nil
}

func (r *SQLiteRepo) CreateSnapshot(ctx context.Context, input domain.CreateSnapshotInput) (domain.Snapshot, error) {
	now := time.Now().UTC()
	
	res, err := r.db.ExecContext(ctx, 
		"INSERT INTO snapshots (license_plate, color, speed, timestamp) VALUES (?, ?, ?, ?)",
		input.LicensePlate, input.Color, input.Speed, now,
	)
	if err != nil {
		return domain.Snapshot{}, err
	}

	id, _ := res.LastInsertId()
	return domain.Snapshot{
		ID:           id,
		LicensePlate: input.LicensePlate,
		Color:        input.Color,
		Speed:        input.Speed,
		Timestamp:    now,
	}, nil
}

func (r *SQLiteRepo) ListSnapshots(ctx context.Context, filter domain.FilterSnapshotsInput) ([]domain.Snapshot, error) {
	query := "SELECT license_plate, color, speed, timestamp FROM snapshots WHERE 1=1"
	var args []interface{}

	if filter.TimeFrom != nil {
		query += " AND timestamp >= ?"
		args = append(args, *filter.TimeFrom)
	}
	if filter.TimeTo != nil {
		query += " AND timestamp <= ?"
		args = append(args, *filter.TimeTo)
	}
	if filter.Color != nil {
		query += " AND color = ?"
		args = append(args, *filter.Color)
	}
	if filter.SpeedFrom != nil {
		query += " AND speed >= ?"
		args = append(args, *filter.SpeedFrom)
	}
	if filter.SpeedTo != nil {
		query += " AND speed <= ?"
		args = append(args, *filter.SpeedTo)
	}

	// Сортируем по возрастанию id
	query += " ORDER BY id ASC"

	rows, err := r.db.QueryContext(ctx, query, args...)
	if err != nil {
		return nil, err
	}
	defer rows.Close()

	var snapshots []domain.Snapshot
	for rows.Next() {
		var s domain.Snapshot
		if err := rows.Scan(&s.LicensePlate, &s.Color, &s.Speed, &s.Timestamp); err != nil {
			return nil, err
		}
		snapshots = append(snapshots, s)
	}

	return snapshots, rows.Err()
}