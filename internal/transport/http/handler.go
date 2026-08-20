package http

import (
	"encoding/json"
	"net/http"
	"strconv"
	"time"

	"speedcamera/internal/domain"
	"speedcamera/internal/service"
)

type Handler struct {
	svc *service.Service
}

func NewHandler(svc *service.Service) *Handler {
	return &Handler{svc: svc}
}

func (h *Handler) RegisterRoutes(mux *http.ServeMux) {
	mux.HandleFunc("/api/v1/snapshots", h.handleSnapshots)
}

func (h *Handler) handleSnapshots(w http.ResponseWriter, r *http.Request) {
	switch r.Method {
	case http.MethodPost:
		h.createSnapshot(w, r)
	case http.MethodGet:
		h.listSnapshots(w, r)
	default:
		http.Error(w, "Method not allowed", http.StatusMethodNotAllowed)
	}
}

func (h *Handler) createSnapshot(w http.ResponseWriter, r *http.Request) {
	var req struct {
		LicensePlate string `json:"license_plate"`
		Color        string `json:"color"`
		Speed        int    `json:"speed"`
	}

	if err := json.NewDecoder(r.Body).Decode(&req); err != nil {
		http.Error(w, "Invalid JSON", http.StatusBadRequest)
		return
	}

	input := domain.CreateSnapshotInput{
		LicensePlate: req.LicensePlate,
		Color:        req.Color,
		Speed:        req.Speed,
	}

	snapshot, err := h.svc.CreateSnapshot(r.Context(), input)
	if err != nil {
		if err == domain.ErrLicensePlateTooLong {
			http.Error(w, err.Error(), http.StatusBadRequest)
			return
		}
		http.Error(w, "Internal server error", http.StatusInternalServerError)
		return
	}

	resp := struct {
		ID        int64     `json:"id"`
		Timestamp time.Time `json:"timestamp"`
	}{
		ID:        snapshot.ID,
		Timestamp: snapshot.Timestamp,
	}

	w.Header().Set("Content-Type", "application/json")
	w.WriteHeader(http.StatusCreated)
	json.NewEncoder(w).Encode(resp)
}

func (h *Handler) listSnapshots(w http.ResponseWriter, r *http.Request) {
	query := r.URL.Query()
	filter := domain.FilterSnapshotsInput{}

	if tf := query.Get("time_from"); tf != "" {
		if t, err := time.Parse(time.RFC3339, tf); err == nil {
			filter.TimeFrom = &t
		}
	}
	if tt := query.Get("time_to"); tt != "" {
		if t, err := time.Parse(time.RFC3339, tt); err == nil {
			filter.TimeTo = &t
		}
	}
	if c := query.Get("color"); c != "" {
		filter.Color = &c
	}
	if sf := query.Get("speed_from"); sf != "" {
		if v, err := strconv.Atoi(sf); err == nil {
			filter.SpeedFrom = &v
		}
	}
	if st := query.Get("speed_to"); st != "" {
		if v, err := strconv.Atoi(st); err == nil {
			filter.SpeedTo = &v
		}
	}

	snapshots, err := h.svc.ListSnapshots(r.Context(), filter)
	if err != nil {
		http.Error(w, "Internal server error", http.StatusInternalServerError)
		return
	}

	// Формируем ответ БЕЗ ID
	type snapshotResp struct {
		LicensePlate string    `json:"license_plate"`
		Color        string    `json:"color"`
		Speed        int       `json:"speed"`
		Timestamp    time.Time `json:"timestamp"`
	}

	resp := make([]snapshotResp, 0, len(snapshots))
	for _, s := range snapshots {
		resp = append(resp, snapshotResp{
			LicensePlate: s.LicensePlate,
			Color:        s.Color,
			Speed:        s.Speed,
			Timestamp:    s.Timestamp,
		})
	}

	w.Header().Set("Content-Type", "application/json")
	json.NewEncoder(w).Encode(resp)
}