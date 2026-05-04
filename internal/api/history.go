package api

import "net/http"

func historyPlaceholderHandler(cfg Config) http.HandlerFunc {
	return func(w http.ResponseWriter, r *http.Request) {
		if cfg.PaidTierEnabled {
			writeJSON(w, http.StatusPaymentRequired, map[string]string{
				"error": "paid history requires auth and billing; those are not implemented in v0.2",
			})
			return
		}
		writeJSON(w, http.StatusPaymentRequired, map[string]string{
			"error": "paid history is not enabled for v0.2",
		})
	}
}
