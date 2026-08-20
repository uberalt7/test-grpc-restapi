package service

import (
	"context"

	"speedcamera/internal/domain"
)

type Service struct {
	repo domain.Repository
}

func NewService(repo domain.Repository) *Service {
	return &Service{repo: repo}
}

func (s *Service) CreateSnapshot(ctx context.Context, input domain.CreateSnapshotInput) (domain.Snapshot, error) {
	// Валидация: номер машины до 10 символов
	if len(input.LicensePlate) > 10 {
		return domain.Snapshot{}, domain.ErrLicensePlateTooLong
	}
	
	return s.repo.CreateSnapshot(ctx, input)
}

func (s *Service) ListSnapshots(ctx context.Context, filter domain.FilterSnapshotsInput) ([]domain.Snapshot, error) {
	return s.repo.ListSnapshots(ctx, filter)
}