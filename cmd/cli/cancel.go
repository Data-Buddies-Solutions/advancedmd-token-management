package main

import (
	"fmt"
	"strconv"

	"github.com/spf13/cobra"
)

func cancelCmd() *cobra.Command {
	var appointmentID string

	cmd := &cobra.Command{
		Use:   "cancel",
		Short: "Cancel an appointment",
		Example: `  amd cancel --appointment-id 98765`,
		RunE: func(cmd *cobra.Command, args []string) error {
			if appointmentID == "" {
				return fmt.Errorf("--appointment-id is required")
			}

			apptIDInt, err := strconv.Atoi(appointmentID)
			if err != nil {
				return fmt.Errorf("appointment-id must be numeric")
			}

			if err := mustBootstrap(); err != nil {
				return err
			}

			tokenData := getToken()

			if err := app.amdRestClient.CancelAppointment(cmd.Context(), tokenData, apptIDInt); err != nil {
				return fmt.Errorf("failed to cancel appointment: %w", err)
			}

			printJSON(map[string]interface{}{
				"status":        "cancelled",
				"appointmentId": apptIDInt,
				"message":       "Appointment cancelled successfully",
			})
			return nil
		},
	}

	cmd.Flags().StringVar(&appointmentID, "appointment-id", "", "Appointment ID to cancel (required)")

	return cmd
}
