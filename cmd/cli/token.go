package main

import (
	"time"

	"github.com/spf13/cobra"
)

func tokenCmd() *cobra.Command {
	return &cobra.Command{
		Use:   "token",
		Short: "Get the current AdvancedMD authentication token",
		RunE: func(cmd *cobra.Command, args []string) error {
			if err := mustBootstrap(); err != nil {
				return err
			}

			resp := getToken().ToResponse()
			nowEST := time.Now().In(eastern)

			printJSON(map[string]interface{}{
				"amd_token":         resp.Token,
				"amd_rest_api_base": resp.RestApiBase,
				"amd_xmlrpc_url":    resp.XmlrpcURL,
				"amd_ehr_api_base":  resp.EhrApiBase,
				"current_date":      nowEST.Format("Monday, January 2, 2006"),
				"current_time":      nowEST.Format("3:04 PM"),
				"created_at":        resp.CreatedAt,
			})
			return nil
		},
	}
}
