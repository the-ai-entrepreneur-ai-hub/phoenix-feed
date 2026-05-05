package api

import (
	"encoding/json"
	"errors"
	"fmt"
	"log/slog"
	"net/http"
	"strconv"
	"strings"
	"time"

	"github.com/abusedmindset/phoenix-feed/internal/store"
)

func activeIncidentsHandler(st Store, cfg Config, log *slog.Logger) http.HandlerFunc {
	return func(w http.ResponseWriter, r *http.Request) {
		writeActiveIncidents(w, r, st, cfg, log)
	}
}

func refreshIncidentsHandler(st Store, cfg Config, log *slog.Logger) http.HandlerFunc {
	return func(w http.ResponseWriter, r *http.Request) {
		writeActiveIncidents(w, r, st, cfg, log)
	}
}

func writeActiveIncidents(w http.ResponseWriter, r *http.Request, st Store, cfg Config, log *slog.Logger) {
	filter, err := parseActiveIncidentFilter(r)
	if err != nil {
		writeError(w, http.StatusBadRequest, err.Error())
		return
	}

	result, err := st.ListActiveIncidents(r.Context(), filter)
	if err != nil {
		log.Error("list active incidents", "err", err)
		writeError(w, http.StatusInternalServerError, "query active incidents")
		return
	}
	if result.Meta.ParserVersion == "" {
		result.Meta.ParserVersion = cfg.DefaultParserVersion
	}

	writeJSON(w, http.StatusOK, result)
}

func parseActiveIncidentFilter(r *http.Request) (store.ActiveIncidentFilter, error) {
	q := r.URL.Query()
	var filter store.ActiveIncidentFilter

	hasBBox := q.Get("bbox") != ""
	hasRadius := q.Get("lat") != "" || q.Get("lon") != "" || q.Get("radius_meters") != ""
	if hasBBox && hasRadius {
		return filter, errors.New("provide either bbox or radius filter, not both")
	}

	if hasBBox {
		bbox, err := parseBBox(q.Get("bbox"))
		if err != nil {
			return filter, err
		}
		filter.BBox = &bbox
	}

	if hasRadius {
		radius, err := parseRadius(q.Get("lat"), q.Get("lon"), q.Get("radius_meters"))
		if err != nil {
			return filter, err
		}
		filter.Radius = &radius
	}

	if sinceRaw := q.Get("since"); sinceRaw != "" {
		since, err := time.Parse(time.RFC3339, sinceRaw)
		if err != nil {
			return filter, fmt.Errorf("since must be RFC3339")
		}
		filter.Since = &since
	}

	if untilRaw := q.Get("until"); untilRaw != "" {
		until, err := time.Parse(time.RFC3339, untilRaw)
		if err != nil {
			return filter, fmt.Errorf("until must be RFC3339")
		}
		filter.Until = &until
	}

	if filter.Since != nil && filter.Until != nil && filter.Since.After(*filter.Until) {
		return filter, errors.New("since must be before until")
	}

	return filter, nil
}

func parseBBox(raw string) (store.BBox, error) {
	parts := strings.Split(raw, ",")
	if len(parts) != 4 {
		return store.BBox{}, errors.New("bbox must be minLon,minLat,maxLon,maxLat")
	}
	values := make([]float64, 4)
	for i, part := range parts {
		v, err := strconv.ParseFloat(strings.TrimSpace(part), 64)
		if err != nil {
			return store.BBox{}, errors.New("bbox values must be numbers")
		}
		values[i] = v
	}
	bbox := store.BBox{MinLon: values[0], MinLat: values[1], MaxLon: values[2], MaxLat: values[3]}
	if bbox.MinLon >= bbox.MaxLon || bbox.MinLat >= bbox.MaxLat {
		return store.BBox{}, errors.New("bbox min values must be less than max values")
	}
	if !validLon(bbox.MinLon) || !validLon(bbox.MaxLon) || !validLat(bbox.MinLat) || !validLat(bbox.MaxLat) {
		return store.BBox{}, errors.New("bbox coordinates out of range")
	}
	return bbox, nil
}

func parseRadius(latRaw, lonRaw, metersRaw string) (store.Radius, error) {
	if latRaw == "" || lonRaw == "" || metersRaw == "" {
		return store.Radius{}, errors.New("radius filter requires lat, lon, and radius_meters")
	}
	lat, err := strconv.ParseFloat(latRaw, 64)
	if err != nil || !validLat(lat) {
		return store.Radius{}, errors.New("lat must be a valid latitude")
	}
	lon, err := strconv.ParseFloat(lonRaw, 64)
	if err != nil || !validLon(lon) {
		return store.Radius{}, errors.New("lon must be a valid longitude")
	}
	meters, err := strconv.ParseFloat(metersRaw, 64)
	if err != nil || meters <= 0 {
		return store.Radius{}, errors.New("radius_meters must be greater than zero")
	}
	return store.Radius{Lat: lat, Lon: lon, Meters: meters}, nil
}

func validLat(v float64) bool {
	return v >= -90 && v <= 90
}

func validLon(v float64) bool {
	return v >= -180 && v <= 180
}

func writeJSON(w http.ResponseWriter, status int, body any) {
	w.Header().Set("Content-Type", "application/json")
	w.WriteHeader(status)
	_ = json.NewEncoder(w).Encode(body)
}

func writeError(w http.ResponseWriter, status int, message string) {
	writeJSON(w, status, map[string]string{"error": message})
}
