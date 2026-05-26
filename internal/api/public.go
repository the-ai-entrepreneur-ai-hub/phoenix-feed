package api

import (
	"log/slog"
	"net/http"

	"github.com/abusedmindset/phoenix-feed/internal/auth"
	"github.com/abusedmindset/phoenix-feed/internal/source/phxfire"
)

func rootHandler() http.HandlerFunc {
	return func(w http.ResponseWriter, r *http.Request) {
		writeJSON(w, http.StatusOK, map[string]string{
			"name":    "cactus-watch-feed",
			"version": "v1",
			"docs":    "https://feed.cactuswatch.com/v1/openapi.json",
			"health":  "https://feed.cactuswatch.com/v1/health",
		})
	}
}

func statsHandler(st Store, cfg Config, log *slog.Logger) http.HandlerFunc {
	return func(w http.ResponseWriter, r *http.Request) {
		stats, err := st.PublicStats(r.Context())
		if err != nil {
			log.Error("public stats", "err", err)
			writeError(w, http.StatusInternalServerError, "query public stats")
			return
		}
		identity := auth.IdentityFromContext(r.Context())
		stats.Tier = string(identity.Tier)
		if stats.Tier == "" {
			stats.Tier = string(auth.TierFree)
		}
		writeJSON(w, http.StatusOK, stats)
	}
}

func codesHandler() http.HandlerFunc {
	return func(w http.ResponseWriter, r *http.Request) {
		writeJSON(w, http.StatusOK, map[string]any{
			"version": phxfire.ParserVersion,
			"codes":   phxfire.CodeDictionary(),
		})
	}
}

func openAPIHandler() http.HandlerFunc {
	return func(w http.ResponseWriter, r *http.Request) {
		writeJSON(w, http.StatusOK, openAPIDocument())
	}
}

func openAPIDocument() map[string]any {
	return map[string]any{
		"openapi": "3.0.3",
		"info": map[string]any{
			"title":   "Cactus Watch Feed API",
			"version": "v1",
		},
		"paths": map[string]any{
			"/": map[string]any{
				"get": map[string]any{"summary": "API identifier"},
			},
			"/v1/incidents/active": map[string]any{
				"get": map[string]any{
					"summary": "List active incidents",
					"parameters": []map[string]any{
						{"name": "bbox", "in": "query", "schema": map[string]string{"type": "string"}},
						{"name": "lat", "in": "query", "schema": map[string]string{"type": "number"}},
						{"name": "lon", "in": "query", "schema": map[string]string{"type": "number"}},
						{"name": "radius_meters", "in": "query", "schema": map[string]string{"type": "number"}},
						{"name": "since", "in": "query", "schema": map[string]string{"type": "string", "format": "date-time"}},
						{"name": "until", "in": "query", "schema": map[string]string{"type": "string", "format": "date-time"}},
					},
					"responses": activeFeedResponses(),
				},
			},
			"/v1/incidents/refresh": map[string]any{
				"post": map[string]any{
					"summary":   "Manual active incident refresh read",
					"responses": activeFeedResponses(),
				},
			},
			"/v1/fire-stations": map[string]any{
				"get": map[string]any{"summary": "List Phoenix Fire station overlay features"},
			},
			"/v1/incidents/{source}/{incident_id}": map[string]any{
				"get": map[string]any{
					"summary": "Get incident detail, unit history, and events",
					"responses": map[string]any{
						"200": map[string]any{
							"description": "Incident detail response",
							"content": map[string]any{
								"application/json": map[string]any{
									"schema": map[string]any{
										"type": "object",
										"properties": map[string]any{
											"meta": map[string]any{"$ref": "#/components/schemas/StalenessMeta"},
										},
									},
								},
							},
						},
					},
				},
			},
			"/v1/incidents/history": map[string]any{
				"get": map[string]any{"summary": "Paid history placeholder"},
			},
			"/api/admin/incidents/recent": map[string]any{
				"get": map[string]any{
					"summary": "Admin-only recent incident history",
					"parameters": []map[string]any{
						{"name": "hours", "in": "query", "schema": map[string]string{"type": "integer"}},
					},
				},
			},
			"/v1/stats": map[string]any{
				"get": map[string]any{"summary": "Public live feed statistics"},
			},
			"/v1/codes": map[string]any{
				"get": map[string]any{"summary": "Phoenix incident code dictionary"},
			},
			"/v1/openapi.json": map[string]any{
				"get": map[string]any{"summary": "OpenAPI document"},
			},
			"/v1/health": map[string]any{
				"get": map[string]any{"summary": "Service health"},
			},
		},
		"components": map[string]any{
			"schemas": map[string]any{
				"StalenessMeta": stalenessMetaSchema(),
			},
		},
	}
}

func activeFeedResponses() map[string]any {
	return map[string]any{
		"200": map[string]any{
			"description": "Active incident feed response",
			"content": map[string]any{
				"application/json": map[string]any{
					"schema": map[string]any{
						"type": "object",
						"properties": map[string]any{
							"meta": map[string]any{"$ref": "#/components/schemas/StalenessMeta"},
							"incidents": map[string]any{
								"type":  "array",
								"items": map[string]any{"type": "object"},
							},
						},
					},
				},
			},
		},
	}
}

func stalenessMetaSchema() map[string]any {
	return map[string]any{
		"type": "object",
		"properties": map[string]any{
			"source_last_success_at": map[string]any{
				"type":        "string",
				"format":      "date-time",
				"nullable":    true,
				"description": "Timestamp of the latest successful upstream scrape.",
			},
			"data_age_seconds": map[string]any{
				"type":        "integer",
				"nullable":    true,
				"description": "Seconds since the latest successful upstream scrape.",
			},
			"newest_incident_at": map[string]any{
				"type":        "string",
				"format":      "date-time",
				"nullable":    true,
				"description": "UTC timestamp of the newest incident_date in this response.",
			},
			"data_staleness_seconds": map[string]any{
				"type":        "integer",
				"nullable":    true,
				"description": "Server-computed seconds between response time and newest_incident_at.",
			},
			"parser_version": map[string]any{
				"type": "string",
			},
			"disclaimer": map[string]any{
				"type": "string",
			},
			"attribution": map[string]any{
				"type": "string",
			},
			"refresh_min_seconds": map[string]any{
				"type": "integer",
			},
			"tier": map[string]any{
				"type": "string",
			},
		},
	}
}
