Dear {{.ReporterName}},

Thank you for submitting your report to {{.PortalName}}. We've received it and will be reviewing it shortly.

{{if .HighPriorityWarning}}
**Priority Notice**: {{.PriorityReason}}. Our team is investigating this urgently.
{{end}}

Use this secure link to access your case (#{{.CaseID}}):

{{.AccessURL}}

This link will expire in {{.ExpiresIn}}. Keep it confidential.

Case Details:
- Reference Number: {{.CaseID}}
- Type: {{.CaseType}}
- Date Submitted: {{.CreatedDate}}
- Status: {{.CaseStatus}}

You can use this link to:
- View case status
- Add comments/information
- Upload files
- See case history

Best regards,
The {{.PortalName}} Team

Reference: {{.CaseID}}
This is an automated email. Do not reply directly.
