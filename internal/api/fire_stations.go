package api

import (
	"encoding/json"
	"fmt"
	"io"
	"log/slog"
	"net/http"
	"net/url"
	"time"
)

const (
	defaultFireStationsURL = "https://maps.phoenix.gov/phxfire/rest/services/SharedResources/PFD_Regional_Dispatch_Fire_Stations/FeatureServer/0/query"
	fireStationsUserAgent  = "cactus-watch-feed/1.0 fire-stations"
)

func fireStationsHandler(cfg Config, log *slog.Logger) http.HandlerFunc {
	upstreamURL := cfg.FireStationsURL
	if upstreamURL == "" {
		upstreamURL = defaultFireStationsURL
	}
	client := cfg.FireStationsHTTPClient
	if client == nil {
		client = &http.Client{Timeout: 15 * time.Second}
	}

	return func(w http.ResponseWriter, r *http.Request) {
		target, err := buildFireStationsURL(upstreamURL, r.URL.Query())
		if err != nil {
			log.Error("fire stations url", "err", err)
			writeError(w, http.StatusInternalServerError, "configure fire stations")
			return
		}

		req, err := http.NewRequestWithContext(r.Context(), http.MethodGet, target, nil)
		if err != nil {
			log.Error("fire stations request", "err", err)
			writeError(w, http.StatusInternalServerError, "build fire stations request")
			return
		}
		req.Header.Set("Accept", "application/json")
		req.Header.Set("User-Agent", fireStationsUserAgent)

		resp, err := client.Do(req)
		if err != nil {
			log.Error("fire stations upstream", "err", err)
			writeError(w, http.StatusBadGateway, "fetch fire stations")
			return
		}
		defer resp.Body.Close()

		body, err := io.ReadAll(io.LimitReader(resp.Body, 5<<20))
		if err != nil {
			log.Error("fire stations read", "err", err)
			writeError(w, http.StatusBadGateway, "read fire stations")
			return
		}
		if resp.StatusCode < 200 || resp.StatusCode >= 300 {
			log.Error("fire stations upstream status", "status", resp.StatusCode)
			writeError(w, http.StatusBadGateway, "fetch fire stations")
			return
		}
		if err := validateFireStationsBody(body); err != nil {
			log.Error("fire stations payload", "err", err)
			writeError(w, http.StatusBadGateway, "parse fire stations")
			return
		}

		w.Header().Set("Content-Type", "application/json")
		w.Header().Set("Cache-Control", "public, max-age=3600")
		w.WriteHeader(http.StatusOK)
		_, _ = w.Write(body)
	}
}

func buildFireStationsURL(raw string, incoming url.Values) (string, error) {
	target, err := url.Parse(raw)
	if err != nil {
		return "", err
	}
	if target.Scheme == "" || target.Host == "" {
		return "", fmt.Errorf("fire stations url must be absolute")
	}

	query := target.Query()
	for key, values := range incoming {
		query.Del(key)
		for _, value := range values {
			query.Add(key, value)
		}
	}
	setDefaultQuery(query, "where", "1=1")
	setDefaultQuery(query, "outFields", "*")
	setDefaultQuery(query, "returnGeometry", "true")
	setDefaultQuery(query, "outSR", "4326")
	setDefaultQuery(query, "f", "json")
	target.RawQuery = query.Encode()
	return target.String(), nil
}

func setDefaultQuery(query url.Values, key string, value string) {
	if query.Get(key) == "" {
		query.Set(key, value)
	}
}

func validateFireStationsBody(body []byte) error {
	var payload struct {
		Features []json.RawMessage `json:"features"`
	}
	if err := json.Unmarshal(body, &payload); err != nil {
		return err
	}
	if payload.Features == nil {
		return fmt.Errorf("missing features array")
	}
	return nil
}
