Dear {{.ReporterName}},

Our system detected {{.FailedCount}} potential issues during the latest scan of your case (#{{.CaseID}}) with {{.PortalName}}.

Scan Details:
- Date: {{.ScanDate}}
- Failed Checks: {{.FailedCount}}
- Case Status: {{.CaseStatus}}

Our team has been notified and will investigate these findings. You can view the latest status of your case here:
{{.CaseURL}}

Best regards,
The {{.PortalName}} Team

Reference: {{.CaseID}}
This is an automated notification. Please do not reply directly.
