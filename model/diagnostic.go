package model

import "fmt"

// DiagnosticLevel indicates whether a diagnostic is an error or warning.
type DiagnosticLevel string

const (
	LevelError   DiagnosticLevel = "error"
	LevelWarning DiagnosticLevel = "warning"
)

// DiagnosticCode represents the stable error/warning codes from spec Section 10.
type DiagnosticCode string

const (
	// Query errors
	CodeQueryInvalidSyntax    DiagnosticCode = "QUERY_INVALID_SYNTAX"
	CodeQueryInvalidOperator  DiagnosticCode = "QUERY_INVALID_OPERATOR"
	CodeQueryORCrossNamespace DiagnosticCode = "QUERY_OR_CROSS_NAMESPACE"
	CodeQueryUnknownField     DiagnosticCode = "QUERY_UNKNOWN_FIELD"
	CodeQueryUnknownFunction  DiagnosticCode = "QUERY_UNKNOWN_FUNCTION"
	CodeQueryInvalidValue     DiagnosticCode = "QUERY_INVALID_VALUE"
	CodeQueryUnqualifiedSLA   DiagnosticCode = "QUERY_UNQUALIFIED_SLA"

	// Path warnings/errors
	CodePathUppercase    DiagnosticCode = "PATH_UPPERCASE"
	CodePathCaseConflict DiagnosticCode = "PATH_CASE_CONFLICT"

	// Redirect errors
	CodeRedirectChainTooDeep  DiagnosticCode = "REDIRECT_CHAIN_TOO_DEEP"
	CodeRedirectInvalidTarget DiagnosticCode = "REDIRECT_INVALID_TARGET"

	// Assignee warnings
	CodeAssigneeUnknownMember  DiagnosticCode = "ASSIGNEE_UNKNOWN_MEMBER"
	CodeAssigneeMemberConflict DiagnosticCode = "ASSIGNEE_MEMBER_CONFLICT"

	// SLA warnings
	CodeSLAStatusMissingUpdatedAt DiagnosticCode = "SLA_STATUS_MISSING_UPDATED_AT"

	// Config errors
	CodeConfigMissing   DiagnosticCode = "CONFIG_MISSING"
	CodeConfigNoVersion DiagnosticCode = "CONFIG_NO_VERSION"
)

// Diagnostic represents a structured error or warning.
type Diagnostic struct {
	Code    DiagnosticCode  `json:"code"`
	Level   DiagnosticLevel `json:"level"`
	Message string          `json:"message"`
	Path    string          `json:"path,omitempty"` // optional: related file path
}

func (d Diagnostic) Error() string {
	return fmt.Sprintf("[%s] %s: %s", d.Level, d.Code, d.Message)
}

// Diagnostics is a collection of diagnostics.
type Diagnostics []Diagnostic

// Errors returns only error-level diagnostics.
func (ds Diagnostics) Errors() Diagnostics {
	var result Diagnostics
	for _, d := range ds {
		if d.Level == LevelError {
			result = append(result, d)
		}
	}
	return result
}

// Warnings returns only warning-level diagnostics.
func (ds Diagnostics) Warnings() Diagnostics {
	var result Diagnostics
	for _, d := range ds {
		if d.Level == LevelWarning {
			result = append(result, d)
		}
	}
	return result
}

// HasErrors returns true if there are any error-level diagnostics.
func (ds Diagnostics) HasErrors() bool {
	for _, d := range ds {
		if d.Level == LevelError {
			return true
		}
	}
	return false
}

// NewError creates an error-level diagnostic.
func NewError(code DiagnosticCode, message string) Diagnostic {
	return Diagnostic{Code: code, Level: LevelError, Message: message}
}

// NewWarning creates a warning-level diagnostic.
func NewWarning(code DiagnosticCode, message string) Diagnostic {
	return Diagnostic{Code: code, Level: LevelWarning, Message: message}
}

// NewErrorf creates an error-level diagnostic with formatted message.
func NewErrorf(code DiagnosticCode, format string, args ...any) Diagnostic {
	return Diagnostic{Code: code, Level: LevelError, Message: fmt.Sprintf(format, args...)}
}

// NewWarningf creates a warning-level diagnostic with formatted message.
func NewWarningf(code DiagnosticCode, format string, args ...any) Diagnostic {
	return Diagnostic{Code: code, Level: LevelWarning, Message: fmt.Sprintf(format, args...)}
}
