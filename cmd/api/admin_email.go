package main

import (
	"log/slog"
	"net/http"
	"time"

	"github.com/priivacy-ai/agent-log-analyzer/internal/app"
)

type adminEmailUnlock struct {
	ID                           string                `json:"id"`
	Email                        string                `json:"email"`
	MarketingOptIn               bool                  `json:"marketing_opt_in"`
	SourceReportJobID            string                `json:"source_report_job_id,omitempty"`
	FullScanJobID                string                `json:"full_scan_job_id,omitempty"`
	Status                       app.EmailUnlockStatus `json:"status"`
	CreatedAt                    time.Time             `json:"created_at"`
	UpdatedAt                    time.Time             `json:"updated_at"`
	ConfirmedAt                  time.Time             `json:"confirmed_at,omitempty"`
	LastTransactionalEmailSentAt time.Time             `json:"last_transactional_email_sent_at,omitempty"`
}

type adminEmailUnlocksResponse struct {
	GeneratedAt  time.Time          `json:"generated_at"`
	Since        time.Time          `json:"since"`
	Until        time.Time          `json:"until"`
	Count        int                `json:"count"`
	UniqueEmails []string           `json:"unique_emails"`
	Records      []adminEmailUnlock `json:"records"`
}

func adminEmailUnlocksHandler(store app.APIStore) http.HandlerFunc {
	return func(w http.ResponseWriter, r *http.Request) {
		if !adminTokenConfigured() {
			writeError(w, http.StatusServiceUnavailable, "admin auth is not configured")
			return
		}
		if !adminTokenMatches(bearerToken(r.Header.Get("Authorization"))) {
			writeError(w, http.StatusUnauthorized, "invalid admin token")
			return
		}
		emailStore, ok := store.(app.EmailUnlockListStore)
		if !ok {
			writeError(w, http.StatusNotImplemented, "email unlock export unavailable")
			return
		}
		since, until, err := usageStatsWindow(r)
		if err != nil {
			writeError(w, http.StatusBadRequest, err.Error())
			return
		}
		unlocks, err := emailStore.ListEmailUnlocks(since, usageStatsLimit(r))
		if err != nil {
			slog.Warn("email unlock export read failed", "error_category", "email_unlock_export_read")
			writeError(w, http.StatusInternalServerError, "could not read email unlocks")
			return
		}
		response := adminEmailUnlocksResponse{
			GeneratedAt: time.Now().UTC(),
			Since:       since,
			Until:       until,
			Records:     make([]adminEmailUnlock, 0, len(unlocks)),
		}
		seen := map[string]struct{}{}
		for _, unlock := range unlocks {
			response.Records = append(response.Records, adminEmailUnlock{
				ID:                           unlock.ID,
				Email:                        unlock.Email,
				MarketingOptIn:               unlock.MarketingOptIn,
				SourceReportJobID:            unlock.SourceReportJobID,
				FullScanJobID:                unlock.FullScanJobID,
				Status:                       unlock.Status,
				CreatedAt:                    unlock.CreatedAt,
				UpdatedAt:                    unlock.UpdatedAt,
				ConfirmedAt:                  unlock.ConfirmedAt,
				LastTransactionalEmailSentAt: unlock.LastTransactionalEmailSentAt,
			})
			if unlock.Email != "" {
				if _, ok := seen[unlock.Email]; !ok {
					seen[unlock.Email] = struct{}{}
					response.UniqueEmails = append(response.UniqueEmails, unlock.Email)
				}
			}
		}
		response.Count = len(response.Records)
		writeJSON(w, http.StatusOK, response)
	}
}
