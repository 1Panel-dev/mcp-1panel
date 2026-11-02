package shared

import (
	"regexp"
	"strings"
	"unicode"
	"unicode/utf8"
)

const (
	WebsiteAliasHint   = "1Panel reports that the website identifier already exists."
	WebsiteDomainHint  = "1Panel reports that the domain is already used by a website."
	PortOccupiedHint   = "1Panel reports that the requested port is already in use."
	DatabaseExistsHint = "1Panel reports that the database or its panel record already exists."
)

var businessHintPatterns = map[string][]*regexp.Regexp{
	WebsiteDomainHint: {
		regexp.MustCompile(`^The domain name is already used by the website \[[^\[\]\r\n]{1,253}\]$`),
		regexp.MustCompile(`^域名已被网站【[^【】\r\n]{1,253}】使用$`),
	},
	PortOccupiedHint: {
		regexp.MustCompile(`^[0-9]{1,5} port is already occupied!$`),
		regexp.MustCompile(`^[0-9]{1,5} 端口已被占用！$`),
		regexp.MustCompile(`^[0-9]{1,5} port is already occupied by application \[[^\[\]\r\n]{1,256}\]!$`),
		regexp.MustCompile(`^[0-9]{1,5} 端口已被应用 \[[^\[\]\r\n]{1,256}\] 占用！$`),
	},
}

// BusinessErrorHint recognizes complete existing API error templates only. It
// returns fixed text, never captured values, and says nothing about commit state.
// Keep the matching fixtures synchronized with Core's private API dispatcher.
func BusinessErrorHint(method, path, message string) string {
	if method != "POST" || len(message) == 0 || len(message) > 2048 || !utf8.ValidString(message) || strings.IndexFunc(message, unicode.IsControl) >= 0 {
		return ""
	}
	for _, prefix := range []string{"Internal server error: ", "服务错误: ", "Bad request: ", "参数错误: "} {
		if strings.HasPrefix(message, prefix) {
			message = strings.TrimPrefix(message, prefix)
			break
		}
	}
	switch path {
	case "/api/v2/websites":
		if message == "Alias already exists" || message == "标识已存在" {
			return WebsiteAliasHint
		}
		for _, pattern := range businessHintPatterns[WebsiteDomainHint] {
			if pattern.MatchString(message) {
				return WebsiteDomainHint
			}
		}
	case "/api/v2/apps/install":
		for _, pattern := range businessHintPatterns[PortOccupiedHint] {
			if pattern.MatchString(message) {
				return PortOccupiedHint
			}
		}
	case "/api/v2/databases":
		if message == "Record already exists" || message == "记录已存在" || message == "Database already exists" || message == "数据库已存在，请重新输入" {
			return DatabaseExistsHint
		}
	}
	return ""
}

func isBusinessHint(hint string) bool {
	switch hint {
	case WebsiteAliasHint, WebsiteDomainHint, PortOccupiedHint, DatabaseExistsHint:
		return true
	}
	return false
}

func UnknownWriteWithHint(hint string) *ExecutionError {
	failure := UnknownWriteResult()
	if isBusinessHint(hint) {
		failure.Message = hint + " " + UnknownWriteMessage
	}
	return failure
}

func IsUnknownWriteMessage(message string) bool {
	if message == UnknownWriteMessage {
		return true
	}
	for _, hint := range []string{WebsiteAliasHint, WebsiteDomainHint, PortOccupiedHint, DatabaseExistsHint} {
		if message == hint+" "+UnknownWriteMessage {
			return true
		}
	}
	return false
}

// SafeExecutionError permits only local fixed reasons with their original error
// classification, or privately constructed validation details. Arbitrary API
// messages cannot enter the response by pretending to be an ExecutionError.
func SafeExecutionError(failure *ExecutionError) (*ExecutionError, bool) {
	if failure == nil {
		return nil, false
	}
	_, _, validation := ValidationDetails(failure)
	safe := validation || (failure.Code == "MCP_EXECUTION_FAILED" && IsUnknownWriteMessage(failure.Message)) || localMessageCodes[failure.Message] == failure.Code && failure.Code != ""
	if !safe {
		return nil, false
	}
	result := *failure
	return &result, true
}

var localMessageCodes = map[string]string{
	NodeAbnormalMessage:                                         "MCP_EXECUTION_FAILED",
	MySQLStoppedMessage:                                         "MCP_EXECUTION_FAILED",
	MySQLConnectionFailedMessage:                                "MCP_EXECUTION_FAILED",
	MySQLStatusFailedMessage:                                    "MCP_EXECUTION_FAILED",
	SystemInfoFailedMessage:                                     "MCP_EXECUTION_FAILED",
	"Invalid tool arguments":                                    "MCP_INVALID_ARGUMENTS",
	"IP certificates support HTTP validation only":              "MCP_INVALID_ARGUMENTS",
	"IP certificates require a Let's Encrypt ACME account":      "MCP_CONFLICT",
	"The requested installed application does not exist":        "MCP_CONFLICT",
	"The requested application does not exist":                  "MCP_CONFLICT",
	"Requested application version is unavailable":              "MCP_CONFLICT",
	"OpenResty is not installed on the selected node":           "MCP_CONFLICT",
	"No default website group exists":                           "MCP_CONFLICT",
	"The requested local MySQL instance is unavailable":         "MCP_CONFLICT",
	"The account name is ambiguous; specify its account ID":     "MCP_CONFLICT",
	"No ACME account exists":                                    "MCP_CONFLICT",
	"More than one ACME account exists; specify its account ID": "MCP_CONFLICT",
	"The requested ACME account does not exist":                 "MCP_CONFLICT",
	"No DNS account exists":                                     "MCP_CONFLICT",
	"More than one DNS account exists; specify its account ID":  "MCP_CONFLICT",
	"The requested DNS account does not exist":                  "MCP_CONFLICT",
	"Tool transport is unavailable":                             "MCP_SCHEMA_MISMATCH",
	"Tool transport or handler is unavailable":                  "MCP_SCHEMA_MISMATCH",
	"Invalid API response":                                      "MCP_SCHEMA_MISMATCH",
	"Query response exceeds its limit":                          "MCP_SCHEMA_MISMATCH",
	"Invalid API operation result":                              "MCP_SCHEMA_MISMATCH",
	"Unexpected application result":                             "MCP_SCHEMA_MISMATCH",
	"Unexpected application detail result":                      "MCP_SCHEMA_MISMATCH",
	"Unexpected installed application result":                   "MCP_SCHEMA_MISMATCH",
	"Invalid application versions result":                       "MCP_SCHEMA_MISMATCH",
	"Invalid application version":                               "MCP_SCHEMA_MISMATCH",
	"Invalid website directory result":                          "MCP_SCHEMA_MISMATCH",
	"Invalid certificate metadata result":                       "MCP_SCHEMA_MISMATCH",
	"Invalid certificate account list":                          "MCP_SCHEMA_MISMATCH",
	"Certificate account list changed during selection":         "MCP_SCHEMA_MISMATCH",
	"Incomplete certificate account list":                       "MCP_SCHEMA_MISMATCH",
	"Certificate account list exceeds its limit":                "MCP_SCHEMA_MISMATCH",
	"Too many MySQL instances":                                  "MCP_SCHEMA_MISMATCH",
	"Invalid MySQL instance result":                             "MCP_SCHEMA_MISMATCH",
	"Invalid MySQL status result":                               "MCP_SCHEMA_MISMATCH",
	"Invalid website metadata result":                           "MCP_SCHEMA_MISMATCH",
	"Website domain result exceeds the supported limit":         "MCP_SCHEMA_MISMATCH",
	"Invalid website domain result":                             "MCP_SCHEMA_MISMATCH",
	"Invalid website HTTPS result":                              "MCP_SCHEMA_MISMATCH",
	"Invalid OpenResty installation result":                     "MCP_SCHEMA_MISMATCH",
	"Invalid OpenResty HTTP port":                               "MCP_SCHEMA_MISMATCH",
}
