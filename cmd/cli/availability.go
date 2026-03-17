package main

import (
	"fmt"
	"log"
	"strconv"
	"strings"
	"time"

	"github.com/spf13/cobra"

	"advancedmd-token-management/internal/domain"
)

// providerDisplayNames maps profile IDs to friendly display names.
var providerDisplayNames = map[string]string{
	"620":  "Dr. Austin Bach",
	"2064": "Dr. J. Licht",
	"2076": "Dr. D. Noel",
}

func availabilityCmd() *cobra.Command {
	var (
		date, provider, office, routing string
		preauthRequired                 bool
	)

	cmd := &cobra.Command{
		Use:   "availability",
		Short: "Check available appointment slots",
		Example: `  amd availability --date 2026-03-20
  amd availability --date 2026-03-20 --provider Bach
  amd availability --date 2026-03-20 --routing bach_only --preauth`,
		RunE: func(cmd *cobra.Command, args []string) error {
			if date == "" {
				return fmt.Errorf("--date is required (YYYY-MM-DD format)")
			}

			startDate, err := time.Parse("2006-01-02", date)
			if err != nil {
				return fmt.Errorf("invalid date format, use YYYY-MM-DD")
			}

			if err := mustBootstrap(); err != nil {
				return err
			}

			// Reject same-day
			todayEastern := time.Now().In(eastern).Format("2006-01-02")
			if startDate.Format("2006-01-02") == todayEastern {
				printJSON(map[string]string{
					"status":  "error",
					"message": "Same-day appointments are not available. Search for tomorrow or later.",
				})
				return nil
			}

			// Preauth enforcement
			if preauthRequired {
				startDate, date = enforcePreauthMinDate(startDate, time.Now().In(eastern))
			}

			log.Printf("checking availability: date=%s provider=%q office=%q routing=%q preauth=%v",
				date, provider, office, routing, preauthRequired)

			tokenData := getToken()

			setup, err := app.amdClient.GetSchedulerSetup(cmd.Context(), tokenData)
			if err != nil {
				return fmt.Errorf("failed to get scheduler setup: %w", err)
			}

			// Build lookup maps
			profileMap := make(map[string]domain.SchedulerProfile)
			for _, p := range setup.Profiles {
				profileMap[p.ID] = p
			}

			facilityMap := make(map[string]domain.SchedulerFacility)
			for _, f := range setup.Facilities {
				facilityMap[f.ID] = f
			}

			// Office filter
			var facilityFilter string
			if office != "" {
				facilityID, ok := domain.LookupFacilityID(office)
				if !ok {
					return fmt.Errorf("unknown office %q — valid: %s", office, strings.Join(domain.ValidOfficeNames(), ", "))
				}
				facilityFilter = facilityID
			}

			// Filter to allowed columns
			var allowedColumns []domain.SchedulerColumn
			for _, col := range setup.Columns {
				if !domain.IsAllowedColumn(col.ID) {
					continue
				}
				if facilityFilter != "" && col.FacilityID != facilityFilter {
					continue
				}
				if provider != "" {
					profile, ok := profileMap[col.ProfileID]
					if !ok {
						continue
					}
					norm := strings.ToUpper(domain.NormalizeForLookup(provider))
					if !strings.Contains(strings.ToUpper(domain.NormalizeForLookup(profile.Name)), norm) &&
						!strings.Contains(strings.ToUpper(domain.NormalizeForLookup(col.Name)), norm) {
						continue
					}
				}
				allowedColumns = append(allowedColumns, col)
			}

			// Routing filter
			if routing != "" {
				rule := domain.ParseRoutingRule(routing)
				routingColumns := domain.ColumnsForRouting(rule)
				if routingColumns != nil {
					var filtered []domain.SchedulerColumn
					for _, col := range allowedColumns {
						if routingColumns[col.ID] {
							filtered = append(filtered, col)
						}
					}
					allowedColumns = filtered
				} else {
					allowedColumns = nil
				}
			}

			// Location name
			locationName := "All Locations"
			if facilityFilter != "" {
				for _, f := range setup.Facilities {
					if f.ID == facilityFilter {
						locationName = f.Name
						break
					}
				}
			} else if len(allowedColumns) > 0 {
				if fac, ok := facilityMap[allowedColumns[0].FacilityID]; ok {
					locationName = fac.Name
				}
			}

			if len(allowedColumns) == 0 {
				if provider != "" {
					return fmt.Errorf("no provider found matching %q — valid: %s", provider, strings.Join(domain.ValidProviderNames(), ", "))
				}
				printJSON(domain.AvailabilityResponse{
					SearchedDate: date,
					Date:         startDate.Format("Monday, January 2, 2006"),
					Location:     locationName,
					Providers:    []domain.ProviderAvailability{},
				})
				return nil
			}

			nowEastern := time.Now().In(eastern)
			searchDate := startDate
			var providers []domain.ProviderAvailability

			for attempt := 0; attempt <= 14; attempt++ {
				dateStr := searchDate.Format("2006-01-02")

				columnIDs := make([]string, len(allowedColumns))
				for i, col := range allowedColumns {
					columnIDs[i] = col.ID
				}

				appointmentsByColumn, err := app.amdRestClient.GetAppointmentsForColumns(cmd.Context(), tokenData, columnIDs, dateStr)
				if err != nil {
					log.Printf("failed to get appointments: %v", err)
					appointmentsByColumn = make(map[string][]domain.Appointment)
				}

				blockHoldsByColumn, err := app.amdRestClient.GetBlockHoldsForColumns(cmd.Context(), tokenData, columnIDs, dateStr)
				if err != nil {
					log.Printf("failed to get block holds: %v", err)
					blockHoldsByColumn = make(map[string][]domain.BlockHold)
				}

				providers = nil
				for _, col := range allowedColumns {
					profile := profileMap[col.ProfileID]
					facility := facilityMap[col.FacilityID]

					displayName := providerDisplayNames[col.ProfileID]
					if displayName == "" {
						displayName = profile.Name
					}

					allSlots := calculateAvailableSlots(col, appointmentsByColumn[col.ID], blockHoldsByColumn[col.ID], searchDate, nowEastern)

					colID, _ := strconv.Atoi(col.ID)
					profID, _ := strconv.Atoi(col.ProfileID)

					pa := domain.ProviderAvailability{
						Name:           displayName,
						ColumnID:       colID,
						ProfileID:      profID,
						Facility:       facility.Name,
						SlotDuration:   col.Interval,
						TotalAvailable: len(allSlots),
					}

					if len(allSlots) > 0 {
						pa.FirstAvailable = allSlots[0].Time
						pa.LastAvailable = allSlots[len(allSlots)-1].Time
						if len(allSlots) > 5 {
							pa.Slots = allSlots[:5]
						} else {
							pa.Slots = allSlots
						}
					} else {
						pa.Slots = []domain.AvailableSlot{}
					}

					providers = append(providers, pa)
				}

				hasAvailability := false
				for _, p := range providers {
					if p.TotalAvailable > 0 {
						hasAvailability = true
						break
					}
				}

				if hasAvailability || attempt == 14 {
					break
				}

				searchDate = searchDate.AddDate(0, 0, 1)
				log.Printf("no slots on %s, searching %s", dateStr, searchDate.Format("2006-01-02"))
			}

			hasAny := false
			for _, p := range providers {
				if p.TotalAvailable > 0 {
					hasAny = true
					break
				}
			}

			if !hasAny {
				printJSON(domain.AvailabilityResponse{
					SearchedDate: date,
					Date:         "",
					Location:     locationName,
					Message:      "No availability found within 14 days of requested date",
					Providers:    []domain.ProviderAvailability{},
				})
				return nil
			}

			printJSON(domain.AvailabilityResponse{
				SearchedDate: date,
				Date:         searchDate.Format("Monday, January 2, 2006"),
				Location:     locationName,
				Providers:    providers,
			})
			return nil
		},
	}

	cmd.Flags().StringVar(&date, "date", "", "Date to check (YYYY-MM-DD, required)")
	cmd.Flags().StringVar(&provider, "provider", "", "Filter by provider name")
	cmd.Flags().StringVar(&office, "office", "", "Filter by office name")
	cmd.Flags().StringVar(&routing, "routing", "", "Routing rule: bach_only, bach_licht, all_three")
	cmd.Flags().BoolVar(&preauthRequired, "preauth", false, "Enforce 14-day minimum lead time")

	return cmd
}

// calculateAvailableSlots generates available time slots for a column on a single day.
func calculateAvailableSlots(col domain.SchedulerColumn, appointments []domain.Appointment, blockHolds []domain.BlockHold, date time.Time, nowEastern time.Time) []domain.AvailableSlot {
	var slots []domain.AvailableSlot

	if !col.WorksOnDay(date.Weekday()) {
		return slots
	}

	workStart, workEnd, err := col.ParseWorkHours(date)
	if err != nil {
		return slots
	}

	today := nowEastern.Format("2006-01-02")
	isToday := date.Format("2006-01-02") == today
	cutoff := nowEastern.Add(30 * time.Minute)

	interval := time.Duration(col.Interval) * time.Minute
	if interval == 0 {
		interval = 15 * time.Minute
	}

	maxAppts := col.MaxApptsPerSlot

	for slotTime := workStart; slotTime.Before(workEnd); slotTime = slotTime.Add(interval) {
		if isToday {
			slotInEastern := time.Date(slotTime.Year(), slotTime.Month(), slotTime.Day(),
				slotTime.Hour(), slotTime.Minute(), 0, 0, nowEastern.Location())
			if slotInEastern.Before(cutoff) {
				continue
			}
		}

		if domain.IsBlockedByHold(slotTime, interval, blockHolds) {
			continue
		}

		if hasOverlappingAppointment(slotTime, appointments) {
			continue
		}

		if maxAppts > 0 {
			count := countSameStartAppointments(slotTime, appointments)
			if count >= maxAppts {
				continue
			}
		}

		slots = append(slots, domain.AvailableSlot{
			Time:     domain.FormatSlotTime(slotTime),
			DateTime: domain.FormatSlotDateTime(slotTime),
		})
	}

	return slots
}

func hasOverlappingAppointment(slotTime time.Time, appointments []domain.Appointment) bool {
	for _, appt := range appointments {
		apptEnd := appt.StartDateTime.Add(time.Duration(appt.Duration) * time.Minute)
		if !slotTime.Before(appt.StartDateTime) && slotTime.Before(apptEnd) {
			return true
		}
	}
	return false
}

func countSameStartAppointments(slotTime time.Time, appointments []domain.Appointment) int {
	count := 0
	for _, appt := range appointments {
		if appt.StartDateTime.Equal(slotTime) {
			count++
		}
	}
	return count
}

func enforcePreauthMinDate(requestedDate time.Time, now time.Time) (time.Time, string) {
	minDate := time.Date(now.Year(), now.Month(), now.Day(), 0, 0, 0, 0, now.Location()).AddDate(0, 0, 14)
	if requestedDate.Before(minDate) {
		log.Printf("preauth required — auto-advanced to %s (14-day minimum)", minDate.Format("2006-01-02"))
		return minDate, minDate.Format("2006-01-02")
	}
	return requestedDate, requestedDate.Format("2006-01-02")
}
