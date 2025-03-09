package email

import (
	"bytes"
	"fmt"
	"time"
)

// ARFSampleGenerator creates test ARF email samples
type ARFSampleGenerator struct {
	humanReadableText string
	machineFields     map[string]string
	originalHeaders   map[string]string
	originalBody      string
	outerHeaders      map[string]string
	boundaryValue     string
}

// NewARFSampleGenerator creates a new sample generator with default values
func NewARFSampleGenerator() *ARFSampleGenerator {
	return &ARFSampleGenerator{
		humanReadableText: "This is a test abuse report.",
		machineFields: map[string]string{
			"Feedback-Type": "abuse",
			"User-Agent":    "Test-Generator/1.0",
			"Version":       "1",
		},
		originalHeaders: map[string]string{
			"From":    "spammer@example.net",
			"To":      "victim@example.com",
			"Subject": "Test Subject",
			"Date":    time.Now().Format(time.RFC1123Z),
		},
		originalBody: "This is the original message body.",
		outerHeaders: map[string]string{
			"From":    "reporter@example.org",
			"To":      "abuse@example.net",
			"Subject": "FW: Test Subject",
			"Date":    time.Now().Format(time.RFC1123Z),
		},
		boundaryValue: "boundary-test-value",
	}
}

// SetHumanReadableText sets the human readable part text
func (g *ARFSampleGenerator) SetHumanReadableText(text string) *ARFSampleGenerator {
	g.humanReadableText = text
	return g
}

// SetFeedbackType sets the feedback type
func (g *ARFSampleGenerator) SetFeedbackType(feedbackType string) *ARFSampleGenerator {
	g.machineFields["Feedback-Type"] = feedbackType
	return g
}

// AddMachineField adds a field to the machine readable part
func (g *ARFSampleGenerator) AddMachineField(name, value string) *ARFSampleGenerator {
	g.machineFields[name] = value
	return g
}

// RemoveMachineField removes a field from the machine readable part
func (g *ARFSampleGenerator) RemoveMachineField(name string) *ARFSampleGenerator {
	delete(g.machineFields, name)
	return g
}

// SetOriginalHeader sets a header in the original message
func (g *ARFSampleGenerator) SetOriginalHeader(name, value string) *ARFSampleGenerator {
	g.originalHeaders[name] = value
	return g
}

// SetOriginalBody sets the body of the original message
func (g *ARFSampleGenerator) SetOriginalBody(body string) *ARFSampleGenerator {
	g.originalBody = body
	return g
}

// SetOuterHeader sets a header in the outer message
func (g *ARFSampleGenerator) SetOuterHeader(name, value string) *ARFSampleGenerator {
	g.outerHeaders[name] = value
	return g
}

// SetBoundary sets the boundary value for multipart sections
func (g *ARFSampleGenerator) SetBoundary(boundary string) *ARFSampleGenerator {
	g.boundaryValue = boundary
	return g
}

// Generate creates an ARF email according to current settings
func (g *ARFSampleGenerator) Generate() string {
	var buf bytes.Buffer

	// Write outer headers
	for k, v := range g.outerHeaders {
		fmt.Fprintf(&buf, "%s: %s\n", k, v)
	}
	fmt.Fprintf(&buf, "MIME-Version: 1.0\n")
	fmt.Fprintf(&buf, "Content-Type: multipart/report; report-type=feedback-report; boundary=\"%s\"\n\n", g.boundaryValue)

	// Part 1: Human readable
	fmt.Fprintf(&buf, "--%s\n", g.boundaryValue)
	fmt.Fprintf(&buf, "Content-Type: text/plain; charset=\"UTF-8\"\n")
	fmt.Fprintf(&buf, "Content-Transfer-Encoding: 7bit\n\n")
	fmt.Fprintf(&buf, "%s\n\n", g.humanReadableText)

	// Part 2: Machine readable
	fmt.Fprintf(&buf, "--%s\n", g.boundaryValue)
	fmt.Fprintf(&buf, "Content-Type: message/feedback-report\n\n")
	for k, v := range g.machineFields {
		fmt.Fprintf(&buf, "%s: %s\n", k, v)
	}
	fmt.Fprintf(&buf, "\n")

	// Part 3: Original email
	fmt.Fprintf(&buf, "--%s\n", g.boundaryValue)
	fmt.Fprintf(&buf, "Content-Type: message/rfc822\n")
	fmt.Fprintf(&buf, "Content-Disposition: inline\n\n")

	for k, v := range g.originalHeaders {
		fmt.Fprintf(&buf, "%s: %s\n", k, v)
	}
	fmt.Fprintf(&buf, "\n%s\n", g.originalBody)

	// Close boundary
	fmt.Fprintf(&buf, "--%s--\n", g.boundaryValue)

	return buf.String()
}

// GenerateWithHeadersOnly creates an ARF with only headers from the original message
func (g *ARFSampleGenerator) GenerateWithHeadersOnly() string {
	var buf bytes.Buffer

	// Write outer headers
	for k, v := range g.outerHeaders {
		fmt.Fprintf(&buf, "%s: %s\n", k, v)
	}
	fmt.Fprintf(&buf, "MIME-Version: 1.0\n")
	fmt.Fprintf(&buf, "Content-Type: multipart/report; report-type=feedback-report; boundary=\"%s\"\n\n", g.boundaryValue)

	// Part 1: Human readable
	fmt.Fprintf(&buf, "--%s\n", g.boundaryValue)
	fmt.Fprintf(&buf, "Content-Type: text/plain; charset=\"UTF-8\"\n")
	fmt.Fprintf(&buf, "Content-Transfer-Encoding: 7bit\n\n")
	fmt.Fprintf(&buf, "%s\n\n", g.humanReadableText)

	// Part 2: Machine readable
	fmt.Fprintf(&buf, "--%s\n", g.boundaryValue)
	fmt.Fprintf(&buf, "Content-Type: message/feedback-report\n\n")
	for k, v := range g.machineFields {
		fmt.Fprintf(&buf, "%s: %s\n", k, v)
	}
	fmt.Fprintf(&buf, "\n")

	// Part 3: Original email headers only
	fmt.Fprintf(&buf, "--%s\n", g.boundaryValue)
	fmt.Fprintf(&buf, "Content-Type: text/rfc822-headers\n")
	fmt.Fprintf(&buf, "Content-Disposition: inline\n\n")

	for k, v := range g.originalHeaders {
		fmt.Fprintf(&buf, "%s: %s\n", k, v)
	}
	fmt.Fprintf(&buf, "\n")

	// Close boundary
	fmt.Fprintf(&buf, "--%s--\n", g.boundaryValue)

	return buf.String()
}

// GenerateInvalid creates various invalid ARF emails
func (g *ARFSampleGenerator) GenerateInvalid(invalidType string) string {
	switch invalidType {
	case "wrong-content-type":
		// Not a multipart/report
		var buf bytes.Buffer
		for k, v := range g.outerHeaders {
			fmt.Fprintf(&buf, "%s: %s\n", k, v)
		}
		fmt.Fprintf(&buf, "MIME-Version: 1.0\n")
		fmt.Fprintf(&buf, "Content-Type: multipart/mixed; boundary=\"%s\"\n\n", g.boundaryValue)

		// Add the same parts but with wrong outer content type
		fmt.Fprintf(&buf, "--%s\n", g.boundaryValue)
		fmt.Fprintf(&buf, "Content-Type: text/plain; charset=\"UTF-8\"\n\n")
		fmt.Fprintf(&buf, "%s\n\n", g.humanReadableText)

		fmt.Fprintf(&buf, "--%s\n", g.boundaryValue)
		fmt.Fprintf(&buf, "Content-Type: message/feedback-report\n\n")
		for k, v := range g.machineFields {
			fmt.Fprintf(&buf, "%s: %s\n", k, v)
		}

		fmt.Fprintf(&buf, "--%s\n", g.boundaryValue)
		fmt.Fprintf(&buf, "Content-Type: message/rfc822\n\n")
		for k, v := range g.originalHeaders {
			fmt.Fprintf(&buf, "%s: %s\n", k, v)
		}
		fmt.Fprintf(&buf, "\n%s\n", g.originalBody)

		fmt.Fprintf(&buf, "--%s--\n", g.boundaryValue)
		return buf.String()

	case "wrong-report-type":
		// Has multipart/report but wrong report-type
		var buf bytes.Buffer
		for k, v := range g.outerHeaders {
			fmt.Fprintf(&buf, "%s: %s\n", k, v)
		}
		fmt.Fprintf(&buf, "MIME-Version: 1.0\n")
		fmt.Fprintf(&buf, "Content-Type: multipart/report; report-type=delivery-status; boundary=\"%s\"\n\n", g.boundaryValue)

		// Add normal parts
		fmt.Fprintf(&buf, "--%s\n", g.boundaryValue)
		fmt.Fprintf(&buf, "Content-Type: text/plain; charset=\"UTF-8\"\n\n")
		fmt.Fprintf(&buf, "%s\n\n", g.humanReadableText)

		fmt.Fprintf(&buf, "--%s\n", g.boundaryValue)
		fmt.Fprintf(&buf, "Content-Type: message/feedback-report\n\n")
		for k, v := range g.machineFields {
			fmt.Fprintf(&buf, "%s: %s\n", k, v)
		}

		fmt.Fprintf(&buf, "--%s\n", g.boundaryValue)
		fmt.Fprintf(&buf, "Content-Type: message/rfc822\n\n")
		for k, v := range g.originalHeaders {
			fmt.Fprintf(&buf, "%s: %s\n", k, v)
		}
		fmt.Fprintf(&buf, "\n%s\n", g.originalBody)

		fmt.Fprintf(&buf, "--%s--\n", g.boundaryValue)
		return buf.String()

	case "missing-feedback-type":
		// Remove required Feedback-Type field
		g.RemoveMachineField("Feedback-Type")
		return g.Generate()

	case "missing-user-agent":
		// Remove required User-Agent field
		g.RemoveMachineField("User-Agent")
		return g.Generate()

	case "missing-version":
		// Remove required Version field
		g.RemoveMachineField("Version")
		return g.Generate()

	case "missing-human-readable":
		// Missing first part (human readable)
		var buf bytes.Buffer
		for k, v := range g.outerHeaders {
			fmt.Fprintf(&buf, "%s: %s\n", k, v)
		}
		fmt.Fprintf(&buf, "MIME-Version: 1.0\n")
		fmt.Fprintf(&buf, "Content-Type: multipart/report; report-type=feedback-report; boundary=\"%s\"\n\n", g.boundaryValue)

		// Skip first part, start with machine readable
		fmt.Fprintf(&buf, "--%s\n", g.boundaryValue)
		fmt.Fprintf(&buf, "Content-Type: message/feedback-report\n\n")
		for k, v := range g.machineFields {
			fmt.Fprintf(&buf, "%s: %s\n", k, v)
		}
		fmt.Fprintf(&buf, "\n")

		// Add original message part
		fmt.Fprintf(&buf, "--%s\n", g.boundaryValue)
		fmt.Fprintf(&buf, "Content-Type: message/rfc822\n\n")
		for k, v := range g.originalHeaders {
			fmt.Fprintf(&buf, "%s: %s\n", k, v)
		}
		fmt.Fprintf(&buf, "\n%s\n", g.originalBody)

		fmt.Fprintf(&buf, "--%s--\n", g.boundaryValue)
		return buf.String()

	case "missing-machine-readable":
		// Missing second part (machine readable)
		var buf bytes.Buffer
		for k, v := range g.outerHeaders {
			fmt.Fprintf(&buf, "%s: %s\n", k, v)
		}
		fmt.Fprintf(&buf, "MIME-Version: 1.0\n")
		fmt.Fprintf(&buf, "Content-Type: multipart/report; report-type=feedback-report; boundary=\"%s\"\n\n", g.boundaryValue)

		// Add human readable part
		fmt.Fprintf(&buf, "--%s\n", g.boundaryValue)
		fmt.Fprintf(&buf, "Content-Type: text/plain; charset=\"UTF-8\"\n\n")
		fmt.Fprintf(&buf, "%s\n\n", g.humanReadableText)

		// Skip machine readable part, go straight to original message
		fmt.Fprintf(&buf, "--%s\n", g.boundaryValue)
		fmt.Fprintf(&buf, "Content-Type: message/rfc822\n\n")
		for k, v := range g.originalHeaders {
			fmt.Fprintf(&buf, "%s: %s\n", k, v)
		}
		fmt.Fprintf(&buf, "\n%s\n", g.originalBody)

		fmt.Fprintf(&buf, "--%s--\n", g.boundaryValue)
		return buf.String()

	case "missing-original-message":
		// Missing third part (original message)
		var buf bytes.Buffer
		for k, v := range g.outerHeaders {
			fmt.Fprintf(&buf, "%s: %s\n", k, v)
		}
		fmt.Fprintf(&buf, "MIME-Version: 1.0\n")
		fmt.Fprintf(&buf, "Content-Type: multipart/report; report-type=feedback-report; boundary=\"%s\"\n\n", g.boundaryValue)

		// Add human readable part
		fmt.Fprintf(&buf, "--%s\n", g.boundaryValue)
		fmt.Fprintf(&buf, "Content-Type: text/plain; charset=\"UTF-8\"\n\n")
		fmt.Fprintf(&buf, "%s\n\n", g.humanReadableText)

		// Add machine readable part
		fmt.Fprintf(&buf, "--%s\n", g.boundaryValue)
		fmt.Fprintf(&buf, "Content-Type: message/feedback-report\n\n")
		for k, v := range g.machineFields {
			fmt.Fprintf(&buf, "%s: %s\n", k, v)
		}
		fmt.Fprintf(&buf, "\n")

		// Skip original message part and close boundary
		fmt.Fprintf(&buf, "--%s--\n", g.boundaryValue)
		return buf.String()

	case "wrong-parts-order":
		// Parts in wrong order
		var buf bytes.Buffer
		for k, v := range g.outerHeaders {
			fmt.Fprintf(&buf, "%s: %s\n", k, v)
		}
		fmt.Fprintf(&buf, "MIME-Version: 1.0\n")
		fmt.Fprintf(&buf, "Content-Type: multipart/report; report-type=feedback-report; boundary=\"%s\"\n\n", g.boundaryValue)

		// First put machine readable (should be second)
		fmt.Fprintf(&buf, "--%s\n", g.boundaryValue)
		fmt.Fprintf(&buf, "Content-Type: message/feedback-report\n\n")
		for k, v := range g.machineFields {
			fmt.Fprintf(&buf, "%s: %s\n", k, v)
		}
		fmt.Fprintf(&buf, "\n")

		// Then human readable (should be first)
		fmt.Fprintf(&buf, "--%s\n", g.boundaryValue)
		fmt.Fprintf(&buf, "Content-Type: text/plain; charset=\"UTF-8\"\n\n")
		fmt.Fprintf(&buf, "%s\n\n", g.humanReadableText)

		// Then original message (correct as third)
		fmt.Fprintf(&buf, "--%s\n", g.boundaryValue)
		fmt.Fprintf(&buf, "Content-Type: message/rfc822\n\n")
		for k, v := range g.originalHeaders {
			fmt.Fprintf(&buf, "%s: %s\n", k, v)
		}
		fmt.Fprintf(&buf, "\n%s\n", g.originalBody)

		fmt.Fprintf(&buf, "--%s--\n", g.boundaryValue)
		return buf.String()

	case "malformed-original-message":
		// Third part present but not parseable
		var buf bytes.Buffer
		for k, v := range g.outerHeaders {
			fmt.Fprintf(&buf, "%s: %s\n", k, v)
		}
		fmt.Fprintf(&buf, "MIME-Version: 1.0\n")
		fmt.Fprintf(&buf, "Content-Type: multipart/report; report-type=feedback-report; boundary=\"%s\"\n\n", g.boundaryValue)

		// Add human readable part
		fmt.Fprintf(&buf, "--%s\n", g.boundaryValue)
		fmt.Fprintf(&buf, "Content-Type: text/plain; charset=\"UTF-8\"\n\n")
		fmt.Fprintf(&buf, "%s\n\n", g.humanReadableText)

		// Add machine readable part
		fmt.Fprintf(&buf, "--%s\n", g.boundaryValue)
		fmt.Fprintf(&buf, "Content-Type: message/feedback-report\n\n")
		for k, v := range g.machineFields {
			fmt.Fprintf(&buf, "%s: %s\n", k, v)
		}
		fmt.Fprintf(&buf, "\n")

		// Add malformed original message part
		fmt.Fprintf(&buf, "--%s\n", g.boundaryValue)
		fmt.Fprintf(&buf, "Content-Type: message/rfc822\n\n")
		fmt.Fprintf(&buf, "This is not a valid email message format\n")
		fmt.Fprintf(&buf, "--%s--\n", g.boundaryValue)
		return buf.String()
	}

	// Default to normal generation
	return g.Generate()
}

// GenerateFromRFCExample generates a sample based on the RFC examples
func GenerateFromRFCExample(useSimple bool) string {
	if useSimple {
		// Based on RFC 5965 Appendix B.1
		return `From: <abusedesk@example.com>
Date: Thu, 8 Mar 2005 17:40:36 EDT
Subject: FW: Earn money
To: <abuse@example.net>
MIME-Version: 1.0
Content-Type: multipart/report; report-type=feedback-report;
     boundary="part1_13d.2e68ed54_boundary"

--part1_13d.2e68ed54_boundary
Content-Type: text/plain; charset="US-ASCII"
Content-Transfer-Encoding: 7bit

This is an email abuse report for an email message received from IP
192.0.2.1 on Thu, 8 Mar 2005 14:00:00 EDT.  For more information
about this format please see http://www.mipassoc.org/arf/.

--part1_13d.2e68ed54_boundary
Content-Type: message/feedback-report

Feedback-Type: abuse
User-Agent: SomeGenerator/1.0
Version: 1

--part1_13d.2e68ed54_boundary
Content-Type: message/rfc822
Content-Disposition: inline

Received: from mailserver.example.net
     (mailserver.example.net [192.0.2.1])
     by example.com with ESMTP id M63d4137594e46;
     Thu, 08 Mar 2005 14:00:00 -0400
From: <somespammer@example.net>
To: <Undisclosed Recipients>
Subject: Earn money
MIME-Version: 1.0
Content-type: text/plain
Message-ID: 8787KJKJ3K4J3K4J3K4J3.mail@example.net
Date: Thu, 02 Sep 2004 12:31:03 -0500

Spam Spam Spam
Spam Spam Spam
Spam Spam Spam
Spam Spam Spam
--part1_13d.2e68ed54_boundary--`
	} else {
		// Based on RFC 5965 Appendix B.2
		return `From: <abusedesk@example.com>
Date: Thu, 8 Mar 2005 17:40:36 EDT
Subject: FW: Earn money
To: <abuse@example.net>
MIME-Version: 1.0
Content-Type: multipart/report; report-type=feedback-report;
     boundary="part1_13d.2e68ed54_boundary"

--part1_13d.2e68ed54_boundary
Content-Type: text/plain; charset="US-ASCII"
Content-Transfer-Encoding: 7bit

This is an email abuse report for an email message received from IP
192.0.2.1 on Thu, 8 Mar 2005 14:00:00 EDT.  For more information
about this format please see http://www.mipassoc.org/arf/.

--part1_13d.2e68ed54_boundary
Content-Type: message/feedback-report

Feedback-Type: abuse
User-Agent: SomeGenerator/1.0
Version: 1
Original-Mail-From: <somespammer@example.net>
Original-Rcpt-To: <user@example.com>
Arrival-Date: Thu, 8 Mar 2005 14:00:00 EDT
Reporting-MTA: dns; mail.example.com
Source-IP: 192.0.2.1
Authentication-Results: mail.example.com;
               spf=fail smtp.mail=somespammer@example.com
Reported-Domain: example.net
Reported-Uri: http://example.net/earn_money.html
Reported-Uri: mailto:user@example.com
Removal-Recipient: user@example.com

--part1_13d.2e68ed54_boundary
Content-Type: message/rfc822
Content-Disposition: inline

From: <somespammer@example.net>
Received: from mailserver.example.net (mailserver.example.net
     [192.0.2.1]) by example.com with ESMTP id M63d4137594e46;
     Thu, 08 Mar 2005 14:00:00 -0400

To: <Undisclosed Recipients>
Subject: Earn money
MIME-Version: 1.0
Content-type: text/plain
Message-ID: 8787KJKJ3K4J3K4J3K4J3.mail@example.net
Date: Thu, 02 Sep 2004 12:31:03 -0500

Spam Spam Spam
Spam Spam Spam
Spam Spam Spam
Spam Spam Spam
--part1_13d.2e68ed54_boundary--`
	}
}

// GenerateWithAllFeedbackTypes generates samples with all defined feedback types
func GenerateWithAllFeedbackTypes() map[string]string {
	feedbackTypes := []string{
		"abuse",
		"fraud",
		"virus",
		"other",
		"not-spam",
		"spam",
		"dkim",
		"spf",
		"tls",
		"auth-failure",
		"harassment",
	}

	result := make(map[string]string)
	for _, feedbackType := range feedbackTypes {
		sample := NewARFSampleGenerator().
			SetFeedbackType(feedbackType).
			Generate()
		result[feedbackType] = sample
	}

	return result
}
