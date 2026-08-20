package grpc

import (
	"context"

	pb "speedcamera/internal/gen/camera"
	"speedcamera/internal/domain"
	"speedcamera/internal/service"
	"google.golang.org/protobuf/types/known/timestamppb"
)

type Server struct {
	pb.UnimplementedCameraServiceServer
	svc *service.Service
}

func NewServer(svc *service.Service) *Server {
	return &Server{svc: svc}
}

func (s *Server) CreateSnapshot(ctx context.Context, req *pb.CreateSnapshotRequest) (*pb.CreateSnapshotResponse, error) {
	input := domain.CreateSnapshotInput{
		LicensePlate: req.LicensePlate,
		Color:        req.Color,
		Speed:        int(req.Speed),
	}

	snapshot, err := s.svc.CreateSnapshot(ctx, input)
	if err != nil {
		return nil, err
	}

	return &pb.CreateSnapshotResponse{
		Id:        snapshot.ID,
		Timestamp: timestamppb.New(snapshot.Timestamp),
	}, nil
}

func (s *Server) ListSnapshots(ctx context.Context, req *pb.ListSnapshotsRequest) (*pb.ListSnapshotsResponse, error) {
	filter := domain.FilterSnapshotsInput{}
	
	if req.Filter != nil {
		if req.Filter.TimeFrom != nil {
			t := req.Filter.TimeFrom.AsTime()
			filter.TimeFrom = &t
		}
		if req.Filter.TimeTo != nil {
			t := req.Filter.TimeTo.AsTime()
			filter.TimeTo = &t
		}
		if req.Filter.Color != "" {
			c := req.Filter.Color
			filter.Color = &c
		}
		if req.Filter.SpeedFrom != 0 {
			sp := int(req.Filter.SpeedFrom)
			filter.SpeedFrom = &sp
		}
		if req.Filter.SpeedTo != 0 {
			sp := int(req.Filter.SpeedTo)
			filter.SpeedTo = &sp
		}
	}

	snapshots, err := s.svc.ListSnapshots(ctx, filter)
	if err != nil {
		return nil, err
	}

	resp := &pb.ListSnapshotsResponse{}
	for _, snap := range snapshots {
		resp.Snapshots = append(resp.Snapshots, &pb.Snapshot{
			LicensePlate: snap.LicensePlate,
			Color:        snap.Color,
			Speed:        int32(snap.Speed),
			Timestamp:    timestamppb.New(snap.Timestamp),
		})
	}

	return resp, nil
}