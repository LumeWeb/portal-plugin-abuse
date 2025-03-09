Dear {{.ReporterName}},

Thank you for submitting your report to {{.PortalName}}. We've received it and will be reviewing it shortly.

{{if .HighPriorityWarning}}
**Priority Notice**: {{.PriorityReason}}. Our team is investigating this urgently.
{{end}}

Report Details:
- Reference Number: {{.CaseID}}
- Type: {{.CaseType}}
- Date Submitted: {{.CreatedDate}}
- Status: {{.CaseStatus}}

You can check the status of your report at:
{{.CaseURL}}

To add more information, reply directly to this email.

Best regards,
The {{.PortalName}} Team

Reference: {{.CaseID}}
This is an automated email. Please do not reply directly.
