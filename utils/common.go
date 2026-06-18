package utils

import (
	"crypto/rand"
	"fmt"
	"math/big"
	"net/url"
	"strings"
)

func GetRandomStr(e int) string {
	value, err := GenerateSecureString(e)
	if err != nil {
		return ""
	}
	return value
}

func GenerateSecureString(length int) (string, error) {
	if length <= 0 {
		return "", fmt.Errorf("length must be greater than zero")
	}

	const charset = "ABCDEFGHJKMNPQRSTWXYZabcdefhijkmnprstwxyz2345678"
	var result strings.Builder
	result.Grow(length)

	max := big.NewInt(int64(len(charset)))
	for i := 0; i < length; i++ {
		index, err := rand.Int(rand.Reader, max)
		if err != nil {
			return "", fmt.Errorf("generate secure random string: %w", err)
		}
		result.WriteByte(charset[index.Int64()])
	}
	return result.String(), nil
}

func GetPortFromAddr(addr string) (string, error) {
	parsedURL, err := url.Parse(addr)
	if err != nil {
		return "", err
	}

	hostPort := parsedURL.Host
	if strings.Contains(hostPort, ":") {
		parts := strings.Split(hostPort, ":")
		return parts[len(parts)-1], nil
	}

	return "", fmt.Errorf("port not found")
}
