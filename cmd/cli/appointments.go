package main

import (
	"fmt"
	"log"
	"strconv"
	"strings"
	"time"

	"github.com/spf13/cobra"

	"advancedmd-token-management/internal/clients"
	"advancedmd-token-management/internal/domain"

	"golang.org/x/text/cases"
	"golang.org/x/text/language"
)

// appointmentTypeNames maps AMD appointment type IDs to friendly names.
var appointmentTypeNames = map[int]string{
	1006: "New Adult Medical",
	1004: "New Pediatric Medical",
	1007: "Established Adult Medical (Follow Up)",
	1005: "Established Pediatric Medical (Follow Up)",
	1008: "Post Op",
}

func appointmentsCmd() *cobra.Command {
	var patientID string

	cmd := &cobra.Command{
		Use:   "appointments",
		Short: "Get upcoming appointments for a patient",
		Example: `  amd appointments --patient-id 12345`,
		RunE: func(cmd *cobra.Command, args []string) error {
			if patientID == "" {
				return fmt.Errorf("--patient-id is required")
			}

			patientIDInt, err := strconv.Atoi(patientID)
			if err != nil {
				return fmt.Errorf("patient-id must be numeric")
			}

			if err := mustBootstrap(); err != nil {
				return err
			}

			tokenData := getToken()

			// Build column ID string for all allowed columns
			var colIDs []string
			for id := range domain.AllowedColumns {
				colIDs = append(colIDs, id)
			}
			columnIDStr := strings.Join(colIDs, "-")

			// Fetch current + next month concurrently
			now := time.Now().In(eastern)
			thisMonth := time.Date(now.Year(), now.Month(), 1, 0, 0, 0, 0, eastern)
			nextMonth := thisMonth.AddDate(0, 1, 0)

			type monthResult struct {
				appts []clients.AMDAppointmentResponse
				err   error
			}
			ch1 := make(chan monthResult, 1)
			ch2 := make(chan monthResult, 1)

			go func() {
				appts, err := app.amdRestClient.GetAppointmentsByMonth(cmd.Context(), tokenData, columnIDStr, thisMonth.Format("2006-01-02"))
				ch1 <- monthResult{appts, err}
			}()
			go func() {
				appts, err := app.amdRestClient.GetAppointmentsByMonth(cmd.Context(), tokenData, columnIDStr, nextMonth.Format("2006-01-02"))
				ch2 <- monthResult{appts, err}
			}()

			r1, r2 := <-ch1, <-ch2

			if r1.err != nil {
				return fmt.Errorf("failed to retrieve appointments (month 1): %w", r1.err)
			}
			if r2.err != nil {
				return fmt.Errorf("failed to retrieve appointments (month 2): %w", r2.err)
			}

			// Combine and filter by patient ID
			allAppts := append(r1.appts, r2.appts...)
			today := time.Date(now.Year(), now.Month(), now.Day(), 0, 0, 0, 0, eastern)

			type apptDetail struct {
				ID        int    `json:"id"`
				Date      string `json:"date"`
				Time      string `json:"time"`
				Provider  string `json:"provider,omitempty"`
				Type      string `json:"type,omitempty"`
				Facility  string `json:"facility,omitempty"`
				Confirmed bool   `json:"confirmed"`
			}

			var details []apptDetail
			for _, a := range allAppts {
				if a.PatientID != patientIDInt {
					continue
				}

				startTime, err := time.Parse("2006-01-02T15:04:05", a.StartDateTime)
				if err != nil {
					startTime, err = time.Parse("2006-01-02T15:04", a.StartDateTime)
					if err != nil {
						continue
					}
				}

				if startTime.Before(today) {
					continue
				}

				typeName := ""
				if len(a.AppointmentTypes) > 0 {
					if name, ok := appointmentTypeNames[a.AppointmentTypes[0]]; ok {
						typeName = name
					}
				}

				details = append(details, apptDetail{
					ID:        a.ID,
					Date:      startTime.Format("Monday, January 2, 2006"),
					Time:      startTime.Format("3:04 PM"),
					Provider:  friendlyProviderName(a.Provider),
					Type:      typeName,
					Facility:  friendlyFacilityName(a.Facility),
					Confirmed: a.ConfirmDate != nil,
				})
			}

			log.Printf("found %d upcoming appointments for patient %s (scanned %d total)", len(details), patientID, len(allAppts))

			if len(details) == 0 {
				printJSON(map[string]interface{}{
					"status":    "no_appointments",
					"patientId": patientID,
					"message":   "No upcoming appointments found",
				})
				return nil
			}

			printJSON(map[string]interface{}{
				"status":       "found",
				"patientId":    patientID,
				"appointments": details,
				"message":      fmt.Sprintf("Found %d upcoming appointment(s)", len(details)),
			})
			return nil
		},
	}

	cmd.Flags().StringVar(&patientID, "patient-id", "", "Patient ID (required)")

	return cmd
}

// friendlyFacilityName cleans up AMD facility names to title case.
func friendlyFacilityName(amdName string) string {
	if amdName == "" {
		return ""
	}
	return cases.Title(language.English).String(strings.ToLower(amdName))
}
