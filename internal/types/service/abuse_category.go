package service

// AbuseCategory represents the category of abuse being reported
type AbuseCategory string

// Abuse categories
const (
	AbuseCategoryMaliciousContent   AbuseCategory = "malicious_content"
	AbuseCategoryResourceAbuse      AbuseCategory = "resource_abuse"
	AbuseCategoryCopyrightViolation AbuseCategory = "copyright_violation"
	AbuseCategoryPhishingScam       AbuseCategory = "phishing_scam"
	AbuseCategoryOther              AbuseCategory = "other"
)
