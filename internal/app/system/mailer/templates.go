// internal/app/system/mailer/templates.go
package mailer

import (
	"bytes"
	"html"
	"html/template"
)

// PasswordResetEmailData contains the data for a password reset email.
type PasswordResetEmailData struct {
	AppName   string
	ResetURL  string
	ExpiryMin int
}

// PasswordResetEmail generates both plain text and HTML versions of a password reset email.
func PasswordResetEmail(data PasswordResetEmailData) (textBody, htmlBody string) {
	// Plain text version
	textBody = "You requested a password reset for your " + data.AppName + " account.\n\n" +
		"Click the link below to reset your password:\n\n" +
		data.ResetURL + "\n\n" +
		"This link will expire in " + itoa(data.ExpiryMin) + " minutes.\n\n" +
		"If you did not request this, you can safely ignore this email."

	// HTML version
	var buf bytes.Buffer
	htmlTmpl.Execute(&buf, data)
	htmlBody = buf.String()

	return textBody, htmlBody
}

// LoginCodeEmailData contains the data for a login code email.
type LoginCodeEmailData struct {
	AppName  string
	Code     string
	MagicURL string
}

// PasswordChangedEmailData contains the data for a password changed confirmation email.
type PasswordChangedEmailData struct {
	AppName  string
	LoginURL string
}

// WelcomeEmailData contains the data for a welcome email sent to new users.
type WelcomeEmailData struct {
	AppName   string
	UserName  string
	LoginURL  string
	Role      string // e.g., "member", "leader", "admin"
	OrgName   string // Organization name (optional)
}

// InvitationEmailData contains the data for an invitation email.
type InvitationEmailData struct {
	AppName       string
	InviterName   string
	RecipientName string
	Role          string
	OrgName       string // Organization name (optional)
	AcceptURL     string
	ExpiresIn     string // e.g., "7 days"
}

// AccountDisabledEmailData contains the data for an account disabled notification.
type AccountDisabledEmailData struct {
	AppName     string
	UserName    string
	Reason      string // Optional reason for disabling
	ContactEmail string
}

// AccountEnabledEmailData contains the data for an account enabled notification.
type AccountEnabledEmailData struct {
	AppName  string
	UserName string
	LoginURL string
}

// NewLoginEmailData contains the data for a new login security notification.
type NewLoginEmailData struct {
	AppName    string
	UserName   string
	Device     string // e.g., "Chrome on Windows"
	IPAddress  string
	Location   string // e.g., "New York, US" (optional)
	LoginTime  string // Formatted timestamp
	LoginURL   string
}

// ResourceAssignedEmailData contains the data for a resource assignment notification.
type ResourceAssignedEmailData struct {
	AppName      string
	UserName     string
	ResourceName string
	ResourceType string // e.g., "game", "survey", "tool"
	GroupName    string
	Instructions string // Optional custom instructions
	LaunchURL    string
	VisibleFrom  string // Optional, formatted date
	VisibleUntil string // Optional, formatted date
}

// MaterialAssignedEmailData contains the data for a material assignment notification.
type MaterialAssignedEmailData struct {
	AppName      string
	UserName     string
	MaterialName string
	MaterialType string // e.g., "document", "guide", "survey"
	Directions   string // Optional custom directions
	AccessURL    string
	VisibleFrom  string // Optional, formatted date
	VisibleUntil string // Optional, formatted date
}

// GroupMembershipEmailData contains the data for a group membership notification.
type GroupMembershipEmailData struct {
	AppName   string
	UserName  string
	GroupName string
	OrgName   string
	Role      string // "leader" or "member"
	GroupURL  string
}

// AnnouncementItem represents a single announcement in a digest.
type AnnouncementItem struct {
	Title   string
	Content string
	Type    string // "info", "warning", "critical"
}

// AnnouncementDigestEmailData contains the data for an announcement digest email.
type AnnouncementDigestEmailData struct {
	AppName       string
	UserName      string
	Announcements []AnnouncementItem
	ViewAllURL    string
}

// LoginCodeEmail generates both plain text and HTML versions of a login code email.
func LoginCodeEmail(data LoginCodeEmailData) (textBody, htmlBody string) {
	// Plain text version
	textBody = "Your " + data.AppName + " login code is: " + data.Code + "\n\n" +
		"Or click here to log in:\n" + data.MagicURL + "\n\n" +
		"This code will expire in 10 minutes.\n\n" +
		"If you did not request this, you can safely ignore this email."

	// HTML version
	var buf bytes.Buffer
	loginCodeHTMLTmpl.Execute(&buf, data)
	htmlBody = buf.String()

	return textBody, htmlBody
}

// PasswordChangedEmail generates both plain text and HTML versions of a password changed confirmation email.
func PasswordChangedEmail(data PasswordChangedEmailData) (textBody, htmlBody string) {
	// Plain text version
	textBody = "Your " + data.AppName + " password has been changed.\n\n" +
		"If you made this change, you can safely ignore this email.\n\n" +
		"If you did NOT make this change, your account may have been compromised. " +
		"Please reset your password immediately by visiting:\n" + data.LoginURL + "\n\n" +
		"For security, we recommend you also review your recent account activity."

	// HTML version
	var buf bytes.Buffer
	passwordChangedHTMLTmpl.Execute(&buf, data)
	htmlBody = buf.String()

	return textBody, htmlBody
}

// WelcomeEmail generates both plain text and HTML versions of a welcome email.
func WelcomeEmail(data WelcomeEmailData) (textBody, htmlBody string) {
	// Plain text version
	textBody = "Welcome to " + data.AppName + ", " + data.UserName + "!\n\n" +
		"Your account has been created"
	if data.OrgName != "" {
		textBody += " for " + data.OrgName
	}
	textBody += " with the role of " + data.Role + ".\n\n" +
		"To get started, log in at:\n" + data.LoginURL + "\n\n" +
		"If you have any questions, please contact your administrator."

	// HTML version
	var buf bytes.Buffer
	welcomeHTMLTmpl.Execute(&buf, data)
	htmlBody = buf.String()

	return textBody, htmlBody
}

// InvitationEmail generates both plain text and HTML versions of an invitation email.
func InvitationEmail(data InvitationEmailData) (textBody, htmlBody string) {
	// Plain text version
	textBody = "Hello " + data.RecipientName + ",\n\n" +
		data.InviterName + " has invited you to join " + data.AppName
	if data.OrgName != "" {
		textBody += " as part of " + data.OrgName
	}
	textBody += ".\n\n" +
		"You will have the role of " + data.Role + ".\n\n" +
		"To accept this invitation, visit:\n" + data.AcceptURL + "\n\n" +
		"This invitation will expire in " + data.ExpiresIn + ".\n\n" +
		"If you did not expect this invitation, you can safely ignore this email."

	// HTML version
	var buf bytes.Buffer
	invitationHTMLTmpl.Execute(&buf, data)
	htmlBody = buf.String()

	return textBody, htmlBody
}

// AccountDisabledEmail generates both plain text and HTML versions of an account disabled notification.
func AccountDisabledEmail(data AccountDisabledEmailData) (textBody, htmlBody string) {
	// Plain text version
	textBody = "Hello " + data.UserName + ",\n\n" +
		"Your " + data.AppName + " account has been disabled.\n\n"
	if data.Reason != "" {
		textBody += "Reason: " + data.Reason + "\n\n"
	}
	textBody += "If you believe this was done in error, please contact your administrator"
	if data.ContactEmail != "" {
		textBody += " at " + data.ContactEmail
	}
	textBody += "."

	// HTML version
	var buf bytes.Buffer
	accountDisabledHTMLTmpl.Execute(&buf, data)
	htmlBody = buf.String()

	return textBody, htmlBody
}

// AccountEnabledEmail generates both plain text and HTML versions of an account enabled notification.
func AccountEnabledEmail(data AccountEnabledEmailData) (textBody, htmlBody string) {
	// Plain text version
	textBody = "Hello " + data.UserName + ",\n\n" +
		"Your " + data.AppName + " account has been enabled.\n\n" +
		"You can now log in at:\n" + data.LoginURL + "\n\n" +
		"If you have any questions, please contact your administrator."

	// HTML version
	var buf bytes.Buffer
	accountEnabledHTMLTmpl.Execute(&buf, data)
	htmlBody = buf.String()

	return textBody, htmlBody
}

// NewLoginEmail generates both plain text and HTML versions of a new login security notification.
func NewLoginEmail(data NewLoginEmailData) (textBody, htmlBody string) {
	// Plain text version
	textBody = "Hello " + data.UserName + ",\n\n" +
		"A new login to your " + data.AppName + " account was detected.\n\n" +
		"Details:\n" +
		"  Device: " + data.Device + "\n" +
		"  IP Address: " + data.IPAddress + "\n"
	if data.Location != "" {
		textBody += "  Location: " + data.Location + "\n"
	}
	textBody += "  Time: " + data.LoginTime + "\n\n" +
		"If this was you, you can safely ignore this email.\n\n" +
		"If this was NOT you, please secure your account immediately by:\n" +
		"1. Changing your password\n" +
		"2. Reviewing your recent activity\n\n" +
		"Visit: " + data.LoginURL

	// HTML version
	var buf bytes.Buffer
	newLoginHTMLTmpl.Execute(&buf, data)
	htmlBody = buf.String()

	return textBody, htmlBody
}

// ResourceAssignedEmail generates both plain text and HTML versions of a resource assignment notification.
func ResourceAssignedEmail(data ResourceAssignedEmailData) (textBody, htmlBody string) {
	// Plain text version
	textBody = "Hello " + data.UserName + ",\n\n" +
		"A new " + data.ResourceType + " has been assigned to your group \"" + data.GroupName + "\".\n\n" +
		"Resource: " + data.ResourceName + "\n"
	if data.VisibleFrom != "" {
		textBody += "Available from: " + data.VisibleFrom + "\n"
	}
	if data.VisibleUntil != "" {
		textBody += "Available until: " + data.VisibleUntil + "\n"
	}
	if data.Instructions != "" {
		textBody += "\nInstructions:\n" + data.Instructions + "\n"
	}
	textBody += "\nAccess it here:\n" + data.LaunchURL

	// HTML version
	var buf bytes.Buffer
	resourceAssignedHTMLTmpl.Execute(&buf, data)
	htmlBody = buf.String()

	return textBody, htmlBody
}

// MaterialAssignedEmail generates both plain text and HTML versions of a material assignment notification.
func MaterialAssignedEmail(data MaterialAssignedEmailData) (textBody, htmlBody string) {
	// Plain text version
	textBody = "Hello " + data.UserName + ",\n\n" +
		"A new " + data.MaterialType + " has been assigned to you.\n\n" +
		"Material: " + data.MaterialName + "\n"
	if data.VisibleFrom != "" {
		textBody += "Available from: " + data.VisibleFrom + "\n"
	}
	if data.VisibleUntil != "" {
		textBody += "Available until: " + data.VisibleUntil + "\n"
	}
	if data.Directions != "" {
		textBody += "\nDirections:\n" + data.Directions + "\n"
	}
	textBody += "\nAccess it here:\n" + data.AccessURL

	// HTML version
	var buf bytes.Buffer
	materialAssignedHTMLTmpl.Execute(&buf, data)
	htmlBody = buf.String()

	return textBody, htmlBody
}

// GroupMembershipEmail generates both plain text and HTML versions of a group membership notification.
func GroupMembershipEmail(data GroupMembershipEmailData) (textBody, htmlBody string) {
	// Plain text version
	textBody = "Hello " + data.UserName + ",\n\n" +
		"You have been added to the group \"" + data.GroupName + "\""
	if data.OrgName != "" {
		textBody += " in " + data.OrgName
	}
	textBody += ".\n\n" +
		"Your role: " + data.Role + "\n\n" +
		"View your group:\n" + data.GroupURL

	// HTML version
	var buf bytes.Buffer
	groupMembershipHTMLTmpl.Execute(&buf, data)
	htmlBody = buf.String()

	return textBody, htmlBody
}

// AnnouncementDigestEmail generates both plain text and HTML versions of an announcement digest email.
func AnnouncementDigestEmail(data AnnouncementDigestEmailData) (textBody, htmlBody string) {
	// Plain text version
	textBody = "Hello " + data.UserName + ",\n\n" +
		"Here are the latest announcements from " + data.AppName + ":\n\n"
	for i, a := range data.Announcements {
		textBody += itoa(i+1) + ". [" + a.Type + "] " + a.Title + "\n"
		textBody += "   " + a.Content + "\n\n"
	}
	textBody += "View all announcements:\n" + data.ViewAllURL

	// HTML version
	var buf bytes.Buffer
	announcementDigestHTMLTmpl.Execute(&buf, data)
	htmlBody = buf.String()

	return textBody, htmlBody
}

func itoa(i int) string {
	if i == 0 {
		return "0"
	}
	var b [20]byte
	n := len(b)
	neg := i < 0
	if neg {
		i = -i
	}
	for i > 0 {
		n--
		b[n] = byte('0' + i%10)
		i /= 10
	}
	if neg {
		n--
		b[n] = '-'
	}
	return string(b[n:])
}

var htmlTmpl = template.Must(template.New("password_reset").Funcs(template.FuncMap{
	"safe": func(s string) template.HTML { return template.HTML(s) },
	"esc":  html.EscapeString,
}).Parse(`<!DOCTYPE html>
<html>
<head>
  <meta charset="UTF-8">
  <meta name="viewport" content="width=device-width, initial-scale=1.0">
  <title>Password Reset</title>
</head>
<body style="margin: 0; padding: 0; font-family: -apple-system, BlinkMacSystemFont, 'Segoe UI', Roboto, 'Helvetica Neue', Arial, sans-serif; background-color: #f4f4f5;">
  <table role="presentation" width="100%" cellspacing="0" cellpadding="0" style="background-color: #f4f4f5;">
    <tr>
      <td align="center" style="padding: 40px 20px;">
        <table role="presentation" width="100%" cellspacing="0" cellpadding="0" style="max-width: 480px; background-color: #ffffff; border-radius: 8px; box-shadow: 0 1px 3px rgba(0,0,0,0.1);">
          <!-- Header -->
          <tr>
            <td style="padding: 32px 32px 24px 32px; text-align: center; border-bottom: 1px solid #e4e4e7;">
              <h1 style="margin: 0; font-size: 24px; font-weight: 600; color: #18181b;">{{.AppName}}</h1>
            </td>
          </tr>
          <!-- Content -->
          <tr>
            <td style="padding: 32px;">
              <h2 style="margin: 0 0 16px 0; font-size: 20px; font-weight: 600; color: #18181b;">Reset Your Password</h2>
              <p style="margin: 0 0 24px 0; font-size: 15px; line-height: 1.6; color: #52525b;">
                You requested a password reset for your account. Click the button below to create a new password.
              </p>
              <!-- Button -->
              <table role="presentation" width="100%" cellspacing="0" cellpadding="0">
                <tr>
                  <td align="center" style="padding: 8px 0 24px 0;">
                    <a href="{{.ResetURL}}" style="display: inline-block; padding: 14px 32px; background-color: #4f46e5; color: #ffffff; text-decoration: none; font-size: 15px; font-weight: 600; border-radius: 6px;">Reset Password</a>
                  </td>
                </tr>
              </table>
              <p style="margin: 0 0 16px 0; font-size: 14px; line-height: 1.6; color: #71717a;">
                This link will expire in <strong>{{.ExpiryMin}} minutes</strong>.
              </p>
              <p style="margin: 0; font-size: 14px; line-height: 1.6; color: #71717a;">
                If you didn't request this password reset, you can safely ignore this email. Your password will remain unchanged.
              </p>
            </td>
          </tr>
          <!-- Footer -->
          <tr>
            <td style="padding: 24px 32px; background-color: #fafafa; border-top: 1px solid #e4e4e7; border-radius: 0 0 8px 8px;">
              <p style="margin: 0 0 8px 0; font-size: 12px; color: #a1a1aa; text-align: center;">
                If the button doesn't work, copy and paste this link into your browser:
              </p>
              <p style="margin: 0; font-size: 12px; color: #4f46e5; text-align: center; word-break: break-all;">
                {{.ResetURL}}
              </p>
            </td>
          </tr>
        </table>
      </td>
    </tr>
  </table>
</body>
</html>`))

var loginCodeHTMLTmpl = template.Must(template.New("login_code").Parse(`<!DOCTYPE html>
<html>
<head>
  <meta charset="UTF-8">
  <meta name="viewport" content="width=device-width, initial-scale=1.0">
  <title>Login Code</title>
</head>
<body style="margin: 0; padding: 0; font-family: -apple-system, BlinkMacSystemFont, 'Segoe UI', Roboto, 'Helvetica Neue', Arial, sans-serif; background-color: #f4f4f5;">
  <table role="presentation" width="100%" cellspacing="0" cellpadding="0" style="background-color: #f4f4f5;">
    <tr>
      <td align="center" style="padding: 40px 20px;">
        <table role="presentation" width="100%" cellspacing="0" cellpadding="0" style="max-width: 480px; background-color: #ffffff; border-radius: 8px; box-shadow: 0 1px 3px rgba(0,0,0,0.1);">
          <!-- Header -->
          <tr>
            <td style="padding: 32px 32px 24px 32px; text-align: center; border-bottom: 1px solid #e4e4e7;">
              <h1 style="margin: 0; font-size: 24px; font-weight: 600; color: #18181b;">{{.AppName}}</h1>
            </td>
          </tr>
          <!-- Content -->
          <tr>
            <td style="padding: 32px;">
              <h2 style="margin: 0 0 16px 0; font-size: 20px; font-weight: 600; color: #18181b;">Your Login Code</h2>
              <p style="margin: 0 0 24px 0; font-size: 15px; line-height: 1.6; color: #52525b;">
                Enter this code to log in to your account:
              </p>
              <!-- Code Box -->
              <table role="presentation" width="100%" cellspacing="0" cellpadding="0">
                <tr>
                  <td align="center" style="padding: 8px 0 24px 0;">
                    <div style="display: inline-block; padding: 16px 32px; background-color: #f4f4f5; border-radius: 8px; font-size: 32px; font-weight: 700; letter-spacing: 4px; color: #18181b;">{{.Code}}</div>
                  </td>
                </tr>
              </table>
              <p style="margin: 0 0 24px 0; font-size: 14px; line-height: 1.6; color: #71717a; text-align: center;">
                Or click the button below to log in automatically:
              </p>
              <!-- Button -->
              <table role="presentation" width="100%" cellspacing="0" cellpadding="0">
                <tr>
                  <td align="center" style="padding: 0 0 24px 0;">
                    <a href="{{.MagicURL}}" style="display: inline-block; padding: 14px 32px; background-color: #4f46e5; color: #ffffff; text-decoration: none; font-size: 15px; font-weight: 600; border-radius: 6px;">Log In</a>
                  </td>
                </tr>
              </table>
              <p style="margin: 0; font-size: 14px; line-height: 1.6; color: #71717a;">
                This code will expire in <strong>10 minutes</strong>. If you didn't request this, you can safely ignore this email.
              </p>
            </td>
          </tr>
          <!-- Footer -->
          <tr>
            <td style="padding: 24px 32px; background-color: #fafafa; border-top: 1px solid #e4e4e7; border-radius: 0 0 8px 8px;">
              <p style="margin: 0 0 8px 0; font-size: 12px; color: #a1a1aa; text-align: center;">
                If the button doesn't work, copy and paste this link into your browser:
              </p>
              <p style="margin: 0; font-size: 12px; color: #4f46e5; text-align: center; word-break: break-all;">
                {{.MagicURL}}
              </p>
            </td>
          </tr>
        </table>
      </td>
    </tr>
  </table>
</body>
</html>`))

var passwordChangedHTMLTmpl = template.Must(template.New("password_changed").Parse(`<!DOCTYPE html>
<html>
<head>
  <meta charset="UTF-8">
  <meta name="viewport" content="width=device-width, initial-scale=1.0">
  <title>Password Changed</title>
</head>
<body style="margin: 0; padding: 0; font-family: -apple-system, BlinkMacSystemFont, 'Segoe UI', Roboto, 'Helvetica Neue', Arial, sans-serif; background-color: #f4f4f5;">
  <table role="presentation" width="100%" cellspacing="0" cellpadding="0" style="background-color: #f4f4f5;">
    <tr>
      <td align="center" style="padding: 40px 20px;">
        <table role="presentation" width="100%" cellspacing="0" cellpadding="0" style="max-width: 480px; background-color: #ffffff; border-radius: 8px; box-shadow: 0 1px 3px rgba(0,0,0,0.1);">
          <!-- Header -->
          <tr>
            <td style="padding: 32px 32px 24px 32px; text-align: center; border-bottom: 1px solid #e4e4e7;">
              <h1 style="margin: 0; font-size: 24px; font-weight: 600; color: #18181b;">{{.AppName}}</h1>
            </td>
          </tr>
          <!-- Content -->
          <tr>
            <td style="padding: 32px;">
              <!-- Warning Icon -->
              <table role="presentation" width="100%" cellspacing="0" cellpadding="0">
                <tr>
                  <td align="center" style="padding: 0 0 16px 0;">
                    <div style="display: inline-block; width: 48px; height: 48px; background-color: #fef3c7; border-radius: 50%; text-align: center; line-height: 48px; font-size: 24px;">&#9888;</div>
                  </td>
                </tr>
              </table>
              <h2 style="margin: 0 0 16px 0; font-size: 20px; font-weight: 600; color: #18181b; text-align: center;">Password Changed</h2>
              <p style="margin: 0 0 24px 0; font-size: 15px; line-height: 1.6; color: #52525b;">
                Your {{.AppName}} password has been successfully changed.
              </p>
              <p style="margin: 0 0 24px 0; font-size: 15px; line-height: 1.6; color: #52525b;">
                <strong>If you made this change</strong>, you can safely ignore this email.
              </p>
              <div style="padding: 16px; background-color: #fef2f2; border-radius: 6px; border-left: 4px solid #ef4444; margin-bottom: 24px;">
                <p style="margin: 0; font-size: 14px; line-height: 1.6; color: #991b1b;">
                  <strong>If you did NOT make this change</strong>, your account may have been compromised. Please reset your password immediately and review your recent account activity.
                </p>
              </div>
              <!-- Button -->
              <table role="presentation" width="100%" cellspacing="0" cellpadding="0">
                <tr>
                  <td align="center" style="padding: 0 0 24px 0;">
                    <a href="{{.LoginURL}}" style="display: inline-block; padding: 14px 32px; background-color: #4f46e5; color: #ffffff; text-decoration: none; font-size: 15px; font-weight: 600; border-radius: 6px;">Go to Login</a>
                  </td>
                </tr>
              </table>
            </td>
          </tr>
          <!-- Footer -->
          <tr>
            <td style="padding: 24px 32px; background-color: #fafafa; border-top: 1px solid #e4e4e7; border-radius: 0 0 8px 8px;">
              <p style="margin: 0; font-size: 12px; color: #a1a1aa; text-align: center;">
                This is an automated security notification. Please do not reply to this email.
              </p>
            </td>
          </tr>
        </table>
      </td>
    </tr>
  </table>
</body>
</html>`))

var welcomeHTMLTmpl = template.Must(template.New("welcome").Parse(`<!DOCTYPE html>
<html>
<head>
  <meta charset="UTF-8">
  <meta name="viewport" content="width=device-width, initial-scale=1.0">
  <title>Welcome</title>
</head>
<body style="margin: 0; padding: 0; font-family: -apple-system, BlinkMacSystemFont, 'Segoe UI', Roboto, 'Helvetica Neue', Arial, sans-serif; background-color: #f4f4f5;">
  <table role="presentation" width="100%" cellspacing="0" cellpadding="0" style="background-color: #f4f4f5;">
    <tr>
      <td align="center" style="padding: 40px 20px;">
        <table role="presentation" width="100%" cellspacing="0" cellpadding="0" style="max-width: 480px; background-color: #ffffff; border-radius: 8px; box-shadow: 0 1px 3px rgba(0,0,0,0.1);">
          <!-- Header -->
          <tr>
            <td style="padding: 32px 32px 24px 32px; text-align: center; border-bottom: 1px solid #e4e4e7;">
              <h1 style="margin: 0; font-size: 24px; font-weight: 600; color: #18181b;">{{.AppName}}</h1>
            </td>
          </tr>
          <!-- Content -->
          <tr>
            <td style="padding: 32px;">
              <!-- Welcome Icon -->
              <table role="presentation" width="100%" cellspacing="0" cellpadding="0">
                <tr>
                  <td align="center" style="padding: 0 0 16px 0;">
                    <div style="display: inline-block; width: 48px; height: 48px; background-color: #dbeafe; border-radius: 50%; text-align: center; line-height: 48px; font-size: 24px;">&#128075;</div>
                  </td>
                </tr>
              </table>
              <h2 style="margin: 0 0 16px 0; font-size: 20px; font-weight: 600; color: #18181b; text-align: center;">Welcome, {{.UserName}}!</h2>
              <p style="margin: 0 0 16px 0; font-size: 15px; line-height: 1.6; color: #52525b;">
                Your account has been created{{if .OrgName}} for <strong>{{.OrgName}}</strong>{{end}}.
              </p>
              <div style="padding: 16px; background-color: #f4f4f5; border-radius: 6px; margin-bottom: 24px;">
                <p style="margin: 0; font-size: 14px; color: #52525b;">
                  <strong>Your role:</strong> {{.Role}}
                </p>
              </div>
              <p style="margin: 0 0 24px 0; font-size: 15px; line-height: 1.6; color: #52525b;">
                Click the button below to log in and get started.
              </p>
              <!-- Button -->
              <table role="presentation" width="100%" cellspacing="0" cellpadding="0">
                <tr>
                  <td align="center" style="padding: 0 0 24px 0;">
                    <a href="{{.LoginURL}}" style="display: inline-block; padding: 14px 32px; background-color: #4f46e5; color: #ffffff; text-decoration: none; font-size: 15px; font-weight: 600; border-radius: 6px;">Log In</a>
                  </td>
                </tr>
              </table>
              <p style="margin: 0; font-size: 14px; line-height: 1.6; color: #71717a;">
                If you have any questions, please contact your administrator.
              </p>
            </td>
          </tr>
          <!-- Footer -->
          <tr>
            <td style="padding: 24px 32px; background-color: #fafafa; border-top: 1px solid #e4e4e7; border-radius: 0 0 8px 8px;">
              <p style="margin: 0; font-size: 12px; color: #a1a1aa; text-align: center;">
                This is an automated message from {{.AppName}}.
              </p>
            </td>
          </tr>
        </table>
      </td>
    </tr>
  </table>
</body>
</html>`))

var invitationHTMLTmpl = template.Must(template.New("invitation").Parse(`<!DOCTYPE html>
<html>
<head>
  <meta charset="UTF-8">
  <meta name="viewport" content="width=device-width, initial-scale=1.0">
  <title>Invitation</title>
</head>
<body style="margin: 0; padding: 0; font-family: -apple-system, BlinkMacSystemFont, 'Segoe UI', Roboto, 'Helvetica Neue', Arial, sans-serif; background-color: #f4f4f5;">
  <table role="presentation" width="100%" cellspacing="0" cellpadding="0" style="background-color: #f4f4f5;">
    <tr>
      <td align="center" style="padding: 40px 20px;">
        <table role="presentation" width="100%" cellspacing="0" cellpadding="0" style="max-width: 480px; background-color: #ffffff; border-radius: 8px; box-shadow: 0 1px 3px rgba(0,0,0,0.1);">
          <!-- Header -->
          <tr>
            <td style="padding: 32px 32px 24px 32px; text-align: center; border-bottom: 1px solid #e4e4e7;">
              <h1 style="margin: 0; font-size: 24px; font-weight: 600; color: #18181b;">{{.AppName}}</h1>
            </td>
          </tr>
          <!-- Content -->
          <tr>
            <td style="padding: 32px;">
              <!-- Invitation Icon -->
              <table role="presentation" width="100%" cellspacing="0" cellpadding="0">
                <tr>
                  <td align="center" style="padding: 0 0 16px 0;">
                    <div style="display: inline-block; width: 48px; height: 48px; background-color: #dcfce7; border-radius: 50%; text-align: center; line-height: 48px; font-size: 24px;">&#128233;</div>
                  </td>
                </tr>
              </table>
              <h2 style="margin: 0 0 16px 0; font-size: 20px; font-weight: 600; color: #18181b; text-align: center;">You're Invited!</h2>
              <p style="margin: 0 0 16px 0; font-size: 15px; line-height: 1.6; color: #52525b;">
                Hello {{.RecipientName}},
              </p>
              <p style="margin: 0 0 16px 0; font-size: 15px; line-height: 1.6; color: #52525b;">
                <strong>{{.InviterName}}</strong> has invited you to join {{.AppName}}{{if .OrgName}} as part of <strong>{{.OrgName}}</strong>{{end}}.
              </p>
              <div style="padding: 16px; background-color: #f4f4f5; border-radius: 6px; margin-bottom: 24px;">
                <p style="margin: 0; font-size: 14px; color: #52525b;">
                  <strong>Your role:</strong> {{.Role}}
                </p>
              </div>
              <!-- Button -->
              <table role="presentation" width="100%" cellspacing="0" cellpadding="0">
                <tr>
                  <td align="center" style="padding: 0 0 16px 0;">
                    <a href="{{.AcceptURL}}" style="display: inline-block; padding: 14px 32px; background-color: #4f46e5; color: #ffffff; text-decoration: none; font-size: 15px; font-weight: 600; border-radius: 6px;">Accept Invitation</a>
                  </td>
                </tr>
              </table>
              <p style="margin: 0 0 16px 0; font-size: 14px; line-height: 1.6; color: #71717a; text-align: center;">
                This invitation will expire in <strong>{{.ExpiresIn}}</strong>.
              </p>
              <p style="margin: 0; font-size: 14px; line-height: 1.6; color: #71717a;">
                If you did not expect this invitation, you can safely ignore this email.
              </p>
            </td>
          </tr>
          <!-- Footer -->
          <tr>
            <td style="padding: 24px 32px; background-color: #fafafa; border-top: 1px solid #e4e4e7; border-radius: 0 0 8px 8px;">
              <p style="margin: 0 0 8px 0; font-size: 12px; color: #a1a1aa; text-align: center;">
                If the button doesn't work, copy and paste this link into your browser:
              </p>
              <p style="margin: 0; font-size: 12px; color: #4f46e5; text-align: center; word-break: break-all;">
                {{.AcceptURL}}
              </p>
            </td>
          </tr>
        </table>
      </td>
    </tr>
  </table>
</body>
</html>`))

var accountDisabledHTMLTmpl = template.Must(template.New("account_disabled").Parse(`<!DOCTYPE html>
<html>
<head>
  <meta charset="UTF-8">
  <meta name="viewport" content="width=device-width, initial-scale=1.0">
  <title>Account Disabled</title>
</head>
<body style="margin: 0; padding: 0; font-family: -apple-system, BlinkMacSystemFont, 'Segoe UI', Roboto, 'Helvetica Neue', Arial, sans-serif; background-color: #f4f4f5;">
  <table role="presentation" width="100%" cellspacing="0" cellpadding="0" style="background-color: #f4f4f5;">
    <tr>
      <td align="center" style="padding: 40px 20px;">
        <table role="presentation" width="100%" cellspacing="0" cellpadding="0" style="max-width: 480px; background-color: #ffffff; border-radius: 8px; box-shadow: 0 1px 3px rgba(0,0,0,0.1);">
          <!-- Header -->
          <tr>
            <td style="padding: 32px 32px 24px 32px; text-align: center; border-bottom: 1px solid #e4e4e7;">
              <h1 style="margin: 0; font-size: 24px; font-weight: 600; color: #18181b;">{{.AppName}}</h1>
            </td>
          </tr>
          <!-- Content -->
          <tr>
            <td style="padding: 32px;">
              <!-- Disabled Icon -->
              <table role="presentation" width="100%" cellspacing="0" cellpadding="0">
                <tr>
                  <td align="center" style="padding: 0 0 16px 0;">
                    <div style="display: inline-block; width: 48px; height: 48px; background-color: #fee2e2; border-radius: 50%; text-align: center; line-height: 48px; font-size: 24px;">&#128683;</div>
                  </td>
                </tr>
              </table>
              <h2 style="margin: 0 0 16px 0; font-size: 20px; font-weight: 600; color: #18181b; text-align: center;">Account Disabled</h2>
              <p style="margin: 0 0 16px 0; font-size: 15px; line-height: 1.6; color: #52525b;">
                Hello {{.UserName}},
              </p>
              <p style="margin: 0 0 24px 0; font-size: 15px; line-height: 1.6; color: #52525b;">
                Your {{.AppName}} account has been disabled by an administrator.
              </p>
              {{if .Reason}}
              <div style="padding: 16px; background-color: #f4f4f5; border-radius: 6px; margin-bottom: 24px;">
                <p style="margin: 0; font-size: 14px; color: #52525b;">
                  <strong>Reason:</strong> {{.Reason}}
                </p>
              </div>
              {{end}}
              <p style="margin: 0; font-size: 14px; line-height: 1.6; color: #71717a;">
                If you believe this was done in error, please contact your administrator{{if .ContactEmail}} at <a href="mailto:{{.ContactEmail}}" style="color: #4f46e5;">{{.ContactEmail}}</a>{{end}}.
              </p>
            </td>
          </tr>
          <!-- Footer -->
          <tr>
            <td style="padding: 24px 32px; background-color: #fafafa; border-top: 1px solid #e4e4e7; border-radius: 0 0 8px 8px;">
              <p style="margin: 0; font-size: 12px; color: #a1a1aa; text-align: center;">
                This is an automated notification from {{.AppName}}.
              </p>
            </td>
          </tr>
        </table>
      </td>
    </tr>
  </table>
</body>
</html>`))

var accountEnabledHTMLTmpl = template.Must(template.New("account_enabled").Parse(`<!DOCTYPE html>
<html>
<head>
  <meta charset="UTF-8">
  <meta name="viewport" content="width=device-width, initial-scale=1.0">
  <title>Account Enabled</title>
</head>
<body style="margin: 0; padding: 0; font-family: -apple-system, BlinkMacSystemFont, 'Segoe UI', Roboto, 'Helvetica Neue', Arial, sans-serif; background-color: #f4f4f5;">
  <table role="presentation" width="100%" cellspacing="0" cellpadding="0" style="background-color: #f4f4f5;">
    <tr>
      <td align="center" style="padding: 40px 20px;">
        <table role="presentation" width="100%" cellspacing="0" cellpadding="0" style="max-width: 480px; background-color: #ffffff; border-radius: 8px; box-shadow: 0 1px 3px rgba(0,0,0,0.1);">
          <!-- Header -->
          <tr>
            <td style="padding: 32px 32px 24px 32px; text-align: center; border-bottom: 1px solid #e4e4e7;">
              <h1 style="margin: 0; font-size: 24px; font-weight: 600; color: #18181b;">{{.AppName}}</h1>
            </td>
          </tr>
          <!-- Content -->
          <tr>
            <td style="padding: 32px;">
              <!-- Enabled Icon -->
              <table role="presentation" width="100%" cellspacing="0" cellpadding="0">
                <tr>
                  <td align="center" style="padding: 0 0 16px 0;">
                    <div style="display: inline-block; width: 48px; height: 48px; background-color: #dcfce7; border-radius: 50%; text-align: center; line-height: 48px; font-size: 24px;">&#9989;</div>
                  </td>
                </tr>
              </table>
              <h2 style="margin: 0 0 16px 0; font-size: 20px; font-weight: 600; color: #18181b; text-align: center;">Account Enabled</h2>
              <p style="margin: 0 0 16px 0; font-size: 15px; line-height: 1.6; color: #52525b;">
                Hello {{.UserName}},
              </p>
              <p style="margin: 0 0 24px 0; font-size: 15px; line-height: 1.6; color: #52525b;">
                Great news! Your {{.AppName}} account has been enabled. You can now log in and access your account.
              </p>
              <!-- Button -->
              <table role="presentation" width="100%" cellspacing="0" cellpadding="0">
                <tr>
                  <td align="center" style="padding: 0 0 24px 0;">
                    <a href="{{.LoginURL}}" style="display: inline-block; padding: 14px 32px; background-color: #4f46e5; color: #ffffff; text-decoration: none; font-size: 15px; font-weight: 600; border-radius: 6px;">Log In</a>
                  </td>
                </tr>
              </table>
              <p style="margin: 0; font-size: 14px; line-height: 1.6; color: #71717a;">
                If you have any questions, please contact your administrator.
              </p>
            </td>
          </tr>
          <!-- Footer -->
          <tr>
            <td style="padding: 24px 32px; background-color: #fafafa; border-top: 1px solid #e4e4e7; border-radius: 0 0 8px 8px;">
              <p style="margin: 0; font-size: 12px; color: #a1a1aa; text-align: center;">
                This is an automated notification from {{.AppName}}.
              </p>
            </td>
          </tr>
        </table>
      </td>
    </tr>
  </table>
</body>
</html>`))

var newLoginHTMLTmpl = template.Must(template.New("new_login").Parse(`<!DOCTYPE html>
<html>
<head>
  <meta charset="UTF-8">
  <meta name="viewport" content="width=device-width, initial-scale=1.0">
  <title>New Login Detected</title>
</head>
<body style="margin: 0; padding: 0; font-family: -apple-system, BlinkMacSystemFont, 'Segoe UI', Roboto, 'Helvetica Neue', Arial, sans-serif; background-color: #f4f4f5;">
  <table role="presentation" width="100%" cellspacing="0" cellpadding="0" style="background-color: #f4f4f5;">
    <tr>
      <td align="center" style="padding: 40px 20px;">
        <table role="presentation" width="100%" cellspacing="0" cellpadding="0" style="max-width: 480px; background-color: #ffffff; border-radius: 8px; box-shadow: 0 1px 3px rgba(0,0,0,0.1);">
          <!-- Header -->
          <tr>
            <td style="padding: 32px 32px 24px 32px; text-align: center; border-bottom: 1px solid #e4e4e7;">
              <h1 style="margin: 0; font-size: 24px; font-weight: 600; color: #18181b;">{{.AppName}}</h1>
            </td>
          </tr>
          <!-- Content -->
          <tr>
            <td style="padding: 32px;">
              <!-- Security Icon -->
              <table role="presentation" width="100%" cellspacing="0" cellpadding="0">
                <tr>
                  <td align="center" style="padding: 0 0 16px 0;">
                    <div style="display: inline-block; width: 48px; height: 48px; background-color: #dbeafe; border-radius: 50%; text-align: center; line-height: 48px; font-size: 24px;">&#128274;</div>
                  </td>
                </tr>
              </table>
              <h2 style="margin: 0 0 16px 0; font-size: 20px; font-weight: 600; color: #18181b; text-align: center;">New Login Detected</h2>
              <p style="margin: 0 0 16px 0; font-size: 15px; line-height: 1.6; color: #52525b;">
                Hello {{.UserName}},
              </p>
              <p style="margin: 0 0 24px 0; font-size: 15px; line-height: 1.6; color: #52525b;">
                A new login to your {{.AppName}} account was detected.
              </p>
              <div style="padding: 16px; background-color: #f4f4f5; border-radius: 6px; margin-bottom: 24px;">
                <table role="presentation" width="100%" cellspacing="0" cellpadding="0">
                  <tr>
                    <td style="padding: 4px 0; font-size: 14px; color: #52525b;"><strong>Device:</strong></td>
                    <td style="padding: 4px 0; font-size: 14px; color: #52525b; text-align: right;">{{.Device}}</td>
                  </tr>
                  <tr>
                    <td style="padding: 4px 0; font-size: 14px; color: #52525b;"><strong>IP Address:</strong></td>
                    <td style="padding: 4px 0; font-size: 14px; color: #52525b; text-align: right;">{{.IPAddress}}</td>
                  </tr>
                  {{if .Location}}
                  <tr>
                    <td style="padding: 4px 0; font-size: 14px; color: #52525b;"><strong>Location:</strong></td>
                    <td style="padding: 4px 0; font-size: 14px; color: #52525b; text-align: right;">{{.Location}}</td>
                  </tr>
                  {{end}}
                  <tr>
                    <td style="padding: 4px 0; font-size: 14px; color: #52525b;"><strong>Time:</strong></td>
                    <td style="padding: 4px 0; font-size: 14px; color: #52525b; text-align: right;">{{.LoginTime}}</td>
                  </tr>
                </table>
              </div>
              <p style="margin: 0 0 16px 0; font-size: 15px; line-height: 1.6; color: #52525b;">
                <strong>If this was you</strong>, you can safely ignore this email.
              </p>
              <div style="padding: 16px; background-color: #fef2f2; border-radius: 6px; border-left: 4px solid #ef4444; margin-bottom: 24px;">
                <p style="margin: 0; font-size: 14px; line-height: 1.6; color: #991b1b;">
                  <strong>If this was NOT you</strong>, please secure your account immediately by changing your password and reviewing your recent activity.
                </p>
              </div>
              <!-- Button -->
              <table role="presentation" width="100%" cellspacing="0" cellpadding="0">
                <tr>
                  <td align="center" style="padding: 0 0 24px 0;">
                    <a href="{{.LoginURL}}" style="display: inline-block; padding: 14px 32px; background-color: #4f46e5; color: #ffffff; text-decoration: none; font-size: 15px; font-weight: 600; border-radius: 6px;">Review Account</a>
                  </td>
                </tr>
              </table>
            </td>
          </tr>
          <!-- Footer -->
          <tr>
            <td style="padding: 24px 32px; background-color: #fafafa; border-top: 1px solid #e4e4e7; border-radius: 0 0 8px 8px;">
              <p style="margin: 0; font-size: 12px; color: #a1a1aa; text-align: center;">
                This is an automated security notification. Please do not reply to this email.
              </p>
            </td>
          </tr>
        </table>
      </td>
    </tr>
  </table>
</body>
</html>`))

var resourceAssignedHTMLTmpl = template.Must(template.New("resource_assigned").Parse(`<!DOCTYPE html>
<html>
<head>
  <meta charset="UTF-8">
  <meta name="viewport" content="width=device-width, initial-scale=1.0">
  <title>New Resource Available</title>
</head>
<body style="margin: 0; padding: 0; font-family: -apple-system, BlinkMacSystemFont, 'Segoe UI', Roboto, 'Helvetica Neue', Arial, sans-serif; background-color: #f4f4f5;">
  <table role="presentation" width="100%" cellspacing="0" cellpadding="0" style="background-color: #f4f4f5;">
    <tr>
      <td align="center" style="padding: 40px 20px;">
        <table role="presentation" width="100%" cellspacing="0" cellpadding="0" style="max-width: 480px; background-color: #ffffff; border-radius: 8px; box-shadow: 0 1px 3px rgba(0,0,0,0.1);">
          <!-- Header -->
          <tr>
            <td style="padding: 32px 32px 24px 32px; text-align: center; border-bottom: 1px solid #e4e4e7;">
              <h1 style="margin: 0; font-size: 24px; font-weight: 600; color: #18181b;">{{.AppName}}</h1>
            </td>
          </tr>
          <!-- Content -->
          <tr>
            <td style="padding: 32px;">
              <!-- Resource Icon -->
              <table role="presentation" width="100%" cellspacing="0" cellpadding="0">
                <tr>
                  <td align="center" style="padding: 0 0 16px 0;">
                    <div style="display: inline-block; width: 48px; height: 48px; background-color: #e0e7ff; border-radius: 50%; text-align: center; line-height: 48px; font-size: 24px;">&#127918;</div>
                  </td>
                </tr>
              </table>
              <h2 style="margin: 0 0 16px 0; font-size: 20px; font-weight: 600; color: #18181b; text-align: center;">New Resource Available</h2>
              <p style="margin: 0 0 16px 0; font-size: 15px; line-height: 1.6; color: #52525b;">
                Hello {{.UserName}},
              </p>
              <p style="margin: 0 0 24px 0; font-size: 15px; line-height: 1.6; color: #52525b;">
                A new {{.ResourceType}} has been assigned to your group <strong>{{.GroupName}}</strong>.
              </p>
              <div style="padding: 16px; background-color: #f4f4f5; border-radius: 6px; margin-bottom: 24px;">
                <p style="margin: 0 0 8px 0; font-size: 16px; font-weight: 600; color: #18181b;">{{.ResourceName}}</p>
                {{if or .VisibleFrom .VisibleUntil}}
                <p style="margin: 0; font-size: 14px; color: #71717a;">
                  {{if .VisibleFrom}}Available from: {{.VisibleFrom}}{{end}}
                  {{if and .VisibleFrom .VisibleUntil}} · {{end}}
                  {{if .VisibleUntil}}Until: {{.VisibleUntil}}{{end}}
                </p>
                {{end}}
              </div>
              {{if .Instructions}}
              <div style="padding: 16px; background-color: #fffbeb; border-radius: 6px; border-left: 4px solid #f59e0b; margin-bottom: 24px;">
                <p style="margin: 0 0 4px 0; font-size: 12px; font-weight: 600; color: #92400e; text-transform: uppercase;">Instructions</p>
                <p style="margin: 0; font-size: 14px; line-height: 1.6; color: #78350f;">{{.Instructions}}</p>
              </div>
              {{end}}
              <!-- Button -->
              <table role="presentation" width="100%" cellspacing="0" cellpadding="0">
                <tr>
                  <td align="center" style="padding: 0 0 24px 0;">
                    <a href="{{.LaunchURL}}" style="display: inline-block; padding: 14px 32px; background-color: #4f46e5; color: #ffffff; text-decoration: none; font-size: 15px; font-weight: 600; border-radius: 6px;">Launch Resource</a>
                  </td>
                </tr>
              </table>
            </td>
          </tr>
          <!-- Footer -->
          <tr>
            <td style="padding: 24px 32px; background-color: #fafafa; border-top: 1px solid #e4e4e7; border-radius: 0 0 8px 8px;">
              <p style="margin: 0; font-size: 12px; color: #a1a1aa; text-align: center;">
                This is an automated notification from {{.AppName}}.
              </p>
            </td>
          </tr>
        </table>
      </td>
    </tr>
  </table>
</body>
</html>`))

var materialAssignedHTMLTmpl = template.Must(template.New("material_assigned").Parse(`<!DOCTYPE html>
<html>
<head>
  <meta charset="UTF-8">
  <meta name="viewport" content="width=device-width, initial-scale=1.0">
  <title>New Material Available</title>
</head>
<body style="margin: 0; padding: 0; font-family: -apple-system, BlinkMacSystemFont, 'Segoe UI', Roboto, 'Helvetica Neue', Arial, sans-serif; background-color: #f4f4f5;">
  <table role="presentation" width="100%" cellspacing="0" cellpadding="0" style="background-color: #f4f4f5;">
    <tr>
      <td align="center" style="padding: 40px 20px;">
        <table role="presentation" width="100%" cellspacing="0" cellpadding="0" style="max-width: 480px; background-color: #ffffff; border-radius: 8px; box-shadow: 0 1px 3px rgba(0,0,0,0.1);">
          <!-- Header -->
          <tr>
            <td style="padding: 32px 32px 24px 32px; text-align: center; border-bottom: 1px solid #e4e4e7;">
              <h1 style="margin: 0; font-size: 24px; font-weight: 600; color: #18181b;">{{.AppName}}</h1>
            </td>
          </tr>
          <!-- Content -->
          <tr>
            <td style="padding: 32px;">
              <!-- Material Icon -->
              <table role="presentation" width="100%" cellspacing="0" cellpadding="0">
                <tr>
                  <td align="center" style="padding: 0 0 16px 0;">
                    <div style="display: inline-block; width: 48px; height: 48px; background-color: #fae8ff; border-radius: 50%; text-align: center; line-height: 48px; font-size: 24px;">&#128218;</div>
                  </td>
                </tr>
              </table>
              <h2 style="margin: 0 0 16px 0; font-size: 20px; font-weight: 600; color: #18181b; text-align: center;">New Material Available</h2>
              <p style="margin: 0 0 16px 0; font-size: 15px; line-height: 1.6; color: #52525b;">
                Hello {{.UserName}},
              </p>
              <p style="margin: 0 0 24px 0; font-size: 15px; line-height: 1.6; color: #52525b;">
                A new {{.MaterialType}} has been assigned to you.
              </p>
              <div style="padding: 16px; background-color: #f4f4f5; border-radius: 6px; margin-bottom: 24px;">
                <p style="margin: 0 0 8px 0; font-size: 16px; font-weight: 600; color: #18181b;">{{.MaterialName}}</p>
                {{if or .VisibleFrom .VisibleUntil}}
                <p style="margin: 0; font-size: 14px; color: #71717a;">
                  {{if .VisibleFrom}}Available from: {{.VisibleFrom}}{{end}}
                  {{if and .VisibleFrom .VisibleUntil}} · {{end}}
                  {{if .VisibleUntil}}Until: {{.VisibleUntil}}{{end}}
                </p>
                {{end}}
              </div>
              {{if .Directions}}
              <div style="padding: 16px; background-color: #fffbeb; border-radius: 6px; border-left: 4px solid #f59e0b; margin-bottom: 24px;">
                <p style="margin: 0 0 4px 0; font-size: 12px; font-weight: 600; color: #92400e; text-transform: uppercase;">Directions</p>
                <p style="margin: 0; font-size: 14px; line-height: 1.6; color: #78350f;">{{.Directions}}</p>
              </div>
              {{end}}
              <!-- Button -->
              <table role="presentation" width="100%" cellspacing="0" cellpadding="0">
                <tr>
                  <td align="center" style="padding: 0 0 24px 0;">
                    <a href="{{.AccessURL}}" style="display: inline-block; padding: 14px 32px; background-color: #4f46e5; color: #ffffff; text-decoration: none; font-size: 15px; font-weight: 600; border-radius: 6px;">Access Material</a>
                  </td>
                </tr>
              </table>
            </td>
          </tr>
          <!-- Footer -->
          <tr>
            <td style="padding: 24px 32px; background-color: #fafafa; border-top: 1px solid #e4e4e7; border-radius: 0 0 8px 8px;">
              <p style="margin: 0; font-size: 12px; color: #a1a1aa; text-align: center;">
                This is an automated notification from {{.AppName}}.
              </p>
            </td>
          </tr>
        </table>
      </td>
    </tr>
  </table>
</body>
</html>`))

var groupMembershipHTMLTmpl = template.Must(template.New("group_membership").Parse(`<!DOCTYPE html>
<html>
<head>
  <meta charset="UTF-8">
  <meta name="viewport" content="width=device-width, initial-scale=1.0">
  <title>Added to Group</title>
</head>
<body style="margin: 0; padding: 0; font-family: -apple-system, BlinkMacSystemFont, 'Segoe UI', Roboto, 'Helvetica Neue', Arial, sans-serif; background-color: #f4f4f5;">
  <table role="presentation" width="100%" cellspacing="0" cellpadding="0" style="background-color: #f4f4f5;">
    <tr>
      <td align="center" style="padding: 40px 20px;">
        <table role="presentation" width="100%" cellspacing="0" cellpadding="0" style="max-width: 480px; background-color: #ffffff; border-radius: 8px; box-shadow: 0 1px 3px rgba(0,0,0,0.1);">
          <!-- Header -->
          <tr>
            <td style="padding: 32px 32px 24px 32px; text-align: center; border-bottom: 1px solid #e4e4e7;">
              <h1 style="margin: 0; font-size: 24px; font-weight: 600; color: #18181b;">{{.AppName}}</h1>
            </td>
          </tr>
          <!-- Content -->
          <tr>
            <td style="padding: 32px;">
              <!-- Group Icon -->
              <table role="presentation" width="100%" cellspacing="0" cellpadding="0">
                <tr>
                  <td align="center" style="padding: 0 0 16px 0;">
                    <div style="display: inline-block; width: 48px; height: 48px; background-color: #d1fae5; border-radius: 50%; text-align: center; line-height: 48px; font-size: 24px;">&#128101;</div>
                  </td>
                </tr>
              </table>
              <h2 style="margin: 0 0 16px 0; font-size: 20px; font-weight: 600; color: #18181b; text-align: center;">Added to Group</h2>
              <p style="margin: 0 0 16px 0; font-size: 15px; line-height: 1.6; color: #52525b;">
                Hello {{.UserName}},
              </p>
              <p style="margin: 0 0 24px 0; font-size: 15px; line-height: 1.6; color: #52525b;">
                You have been added to a group{{if .OrgName}} in <strong>{{.OrgName}}</strong>{{end}}.
              </p>
              <div style="padding: 16px; background-color: #f4f4f5; border-radius: 6px; margin-bottom: 24px;">
                <p style="margin: 0 0 8px 0; font-size: 16px; font-weight: 600; color: #18181b;">{{.GroupName}}</p>
                <p style="margin: 0; font-size: 14px; color: #71717a;">
                  Your role: <strong>{{.Role}}</strong>
                </p>
              </div>
              <!-- Button -->
              <table role="presentation" width="100%" cellspacing="0" cellpadding="0">
                <tr>
                  <td align="center" style="padding: 0 0 24px 0;">
                    <a href="{{.GroupURL}}" style="display: inline-block; padding: 14px 32px; background-color: #4f46e5; color: #ffffff; text-decoration: none; font-size: 15px; font-weight: 600; border-radius: 6px;">View Group</a>
                  </td>
                </tr>
              </table>
            </td>
          </tr>
          <!-- Footer -->
          <tr>
            <td style="padding: 24px 32px; background-color: #fafafa; border-top: 1px solid #e4e4e7; border-radius: 0 0 8px 8px;">
              <p style="margin: 0; font-size: 12px; color: #a1a1aa; text-align: center;">
                This is an automated notification from {{.AppName}}.
              </p>
            </td>
          </tr>
        </table>
      </td>
    </tr>
  </table>
</body>
</html>`))

var announcementDigestHTMLTmpl = template.Must(template.New("announcement_digest").Parse(`<!DOCTYPE html>
<html>
<head>
  <meta charset="UTF-8">
  <meta name="viewport" content="width=device-width, initial-scale=1.0">
  <title>Announcements</title>
</head>
<body style="margin: 0; padding: 0; font-family: -apple-system, BlinkMacSystemFont, 'Segoe UI', Roboto, 'Helvetica Neue', Arial, sans-serif; background-color: #f4f4f5;">
  <table role="presentation" width="100%" cellspacing="0" cellpadding="0" style="background-color: #f4f4f5;">
    <tr>
      <td align="center" style="padding: 40px 20px;">
        <table role="presentation" width="100%" cellspacing="0" cellpadding="0" style="max-width: 480px; background-color: #ffffff; border-radius: 8px; box-shadow: 0 1px 3px rgba(0,0,0,0.1);">
          <!-- Header -->
          <tr>
            <td style="padding: 32px 32px 24px 32px; text-align: center; border-bottom: 1px solid #e4e4e7;">
              <h1 style="margin: 0; font-size: 24px; font-weight: 600; color: #18181b;">{{.AppName}}</h1>
            </td>
          </tr>
          <!-- Content -->
          <tr>
            <td style="padding: 32px;">
              <!-- Announcement Icon -->
              <table role="presentation" width="100%" cellspacing="0" cellpadding="0">
                <tr>
                  <td align="center" style="padding: 0 0 16px 0;">
                    <div style="display: inline-block; width: 48px; height: 48px; background-color: #fef3c7; border-radius: 50%; text-align: center; line-height: 48px; font-size: 24px;">&#128227;</div>
                  </td>
                </tr>
              </table>
              <h2 style="margin: 0 0 16px 0; font-size: 20px; font-weight: 600; color: #18181b; text-align: center;">Latest Announcements</h2>
              <p style="margin: 0 0 24px 0; font-size: 15px; line-height: 1.6; color: #52525b;">
                Hello {{.UserName}}, here are the latest announcements:
              </p>
              {{range .Announcements}}
              <div style="padding: 16px; margin-bottom: 16px; border-radius: 6px; {{if eq .Type "critical"}}background-color: #fef2f2; border-left: 4px solid #ef4444;{{else if eq .Type "warning"}}background-color: #fffbeb; border-left: 4px solid #f59e0b;{{else}}background-color: #f0f9ff; border-left: 4px solid #3b82f6;{{end}}">
                <p style="margin: 0 0 4px 0; font-size: 12px; font-weight: 600; text-transform: uppercase; {{if eq .Type "critical"}}color: #991b1b;{{else if eq .Type "warning"}}color: #92400e;{{else}}color: #1e40af;{{end}}">{{.Type}}</p>
                <p style="margin: 0 0 8px 0; font-size: 15px; font-weight: 600; color: #18181b;">{{.Title}}</p>
                <p style="margin: 0; font-size: 14px; line-height: 1.5; color: #52525b;">{{.Content}}</p>
              </div>
              {{end}}
              <!-- Button -->
              <table role="presentation" width="100%" cellspacing="0" cellpadding="0">
                <tr>
                  <td align="center" style="padding: 8px 0 24px 0;">
                    <a href="{{.ViewAllURL}}" style="display: inline-block; padding: 14px 32px; background-color: #4f46e5; color: #ffffff; text-decoration: none; font-size: 15px; font-weight: 600; border-radius: 6px;">View All Announcements</a>
                  </td>
                </tr>
              </table>
            </td>
          </tr>
          <!-- Footer -->
          <tr>
            <td style="padding: 24px 32px; background-color: #fafafa; border-top: 1px solid #e4e4e7; border-radius: 0 0 8px 8px;">
              <p style="margin: 0; font-size: 12px; color: #a1a1aa; text-align: center;">
                This is an automated notification from {{.AppName}}.
              </p>
            </td>
          </tr>
        </table>
      </td>
    </tr>
  </table>
</body>
</html>`))
