package shared

import (
	"errors"
	"regexp"
	"strings"
	"unicode"
	"unicode/utf8"
)

const (
	NodeAbnormalMessage          = "1Panel reports an abnormal status for the selected node. Check the node status, Agent service and panel logs."
	MySQLStoppedMessage          = "The selected MySQL instance is not running. Start the database service and try the query again."
	MySQLConnectionFailedMessage = "Cannot connect to the selected MySQL instance. Check the database service, connection settings and panel logs."
	MySQLStatusFailedMessage     = "Cannot read the selected MySQL instance status. Check that the database service is running and review its connection settings and panel logs."
	SystemInfoFailedMessage      = "Cannot read system information from the selected 1Panel node. Check the Agent service and panel logs."
)

var mysqlStoppedPattern = regexp.MustCompile(`^Error response from daemon: [Cc]ontainer [A-Za-z0-9][A-Za-z0-9_.-]{0,127} is not running$`)

var mysqlConnectionPatterns = []*regexp.Regexp{
	regexp.MustCompile(`^ERROR 2002 \(HY000\): Can't connect to local MySQL server through socket '[^'\r\n]{1,1000}' \([0-9]{1,10}\)$`),
	regexp.MustCompile(`^ERROR 2003 \(HY000\): Can't connect to MySQL server on '[^'\r\n]{1,1000}' \([0-9]{1,10}\)$`),
	regexp.MustCompile(`^ERROR 1045 \(28000\): Access denied for user '[^'\r\n]{1,128}'@'[^'\r\n]{1,255}' \(using password: (YES|NO)\)$`),
}

// ReadErrorHint recognizes complete error templates for two read operations.
// Callers must first classify the response as an execution failure: text alone
// must never override authentication, permission, validation or schema errors.
// Only fixed text is returned; captured container IDs, paths and users are not.
// Keep the matching fixtures synchronized with Core and the Agent read runner.
func ReadErrorHint(method, fullAPIPath, message string) string {
	if len(message) == 0 || len(message) > 2048 || !utf8.ValidString(message) {
		return ""
	}
	// Command stderr commonly has one trailing line ending. Do not use TrimSpace:
	// extra lines, arbitrary padding and embedded control characters stay invalid.
	if strings.HasSuffix(message, "\n") {
		message = strings.TrimSuffix(message, "\n")
		message = strings.TrimSuffix(message, "\r")
	}
	for _, prefix := range []string{"Internal server error: ", "服务错误: ", "Bad request: ", "参数错误: "} {
		if strings.HasPrefix(message, prefix) {
			message = strings.TrimPrefix(message, prefix)
			break
		}
	}
	if method == "POST" && fullAPIPath == "/api/v2/databases/status" {
		message = strings.TrimPrefix(message, "mysql: [Warning] Using a password on the command line interface can be insecure.\n")
	}
	if strings.IndexFunc(message, unicode.IsControl) >= 0 {
		return ""
	}
	switch {
	case method == "GET" && fullAPIPath == "/api/v2/dashboard/base/os":
		if message == NodeAbnormalMessage || message == SystemInfoFailedMessage {
			return message
		}
	case method == "POST" && fullAPIPath == "/api/v2/databases/status":
		switch message {
		case NodeAbnormalMessage, MySQLStoppedMessage, MySQLConnectionFailedMessage, MySQLStatusFailedMessage:
			return message
		}
		if mysqlStoppedPattern.MatchString(message) {
			return MySQLStoppedMessage
		}
		for _, pattern := range mysqlConnectionPatterns {
			if pattern.MatchString(message) {
				return MySQLConnectionFailedMessage
			}
		}
	}
	return ""
}

// ReadFailure adds a safe, operation-specific explanation without changing error
// classification or metadata. It does not infer a stopped instance from a 500,
// and never reclassifies an uncertain write as a read failure.
func ReadFailure(err error, method, fullAPIPath string) error {
	var failure *ExecutionError
	if !errors.As(err, &failure) || failure == nil || failure.Code != "MCP_EXECUTION_FAILED" || IsUnknownWriteMessage(failure.Message) {
		return err
	}
	var fallback string
	switch {
	case method == "GET" && fullAPIPath == "/api/v2/dashboard/base/os":
		fallback = SystemInfoFailedMessage
	case method == "POST" && fullAPIPath == "/api/v2/databases/status":
		fallback = MySQLStatusFailedMessage
	default:
		return err
	}
	result := *failure
	result.Message = ReadErrorHint(method, fullAPIPath, failure.Message)
	if result.Message == "" {
		result.Message = fallback
	}
	return &result
}
