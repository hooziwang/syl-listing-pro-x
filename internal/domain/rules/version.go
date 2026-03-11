package rules

import (
	cryptorand "crypto/rand"
	"fmt"
	"math/big"
	"regexp"
	"strings"
	"time"
)

func GenerateVersion(tenant string) string {
	now := time.Now().UTC()
	return fmt.Sprintf("rules-%s-%s-%s", normalizeTenant(tenant), now.Format("20060102-150405"), randomVersionSuffix())
}

func randomVersionSuffix() string {
	const digits = "0123456789abcdefghijklmnopqrstuvwxyz"
	const letters = "abcdefghijklmnopqrstuvwxyz"
	buf := make([]byte, 6)
	hasLetter := false
	for i := range buf {
		n, err := cryptorand.Int(cryptorand.Reader, big36)
		if err != nil {
			idx := int((time.Now().UnixNano() + int64(i)) % int64(len(digits)))
			buf[i] = digits[idx]
		} else {
			buf[i] = digits[int(n.Int64())]
		}
		if buf[i] >= 'a' && buf[i] <= 'z' {
			hasLetter = true
		}
	}
	if !hasLetter {
		n, err := cryptorand.Int(cryptorand.Reader, big26)
		if err != nil {
			idx := int(time.Now().UnixNano() % int64(len(letters)))
			buf[len(buf)-1] = letters[idx]
		} else {
			buf[len(buf)-1] = letters[int(n.Int64())]
		}
	}
	return string(buf)
}

var (
	big36 = big.NewInt(36)
	big26 = big.NewInt(26)
)

func normalizeTenant(raw string) string {
	tenant := strings.ToLower(strings.TrimSpace(raw))
	tenant = regexp.MustCompile(`[^a-z0-9-]+`).ReplaceAllString(tenant, "-")
	tenant = regexp.MustCompile(`-+`).ReplaceAllString(tenant, "-")
	tenant = strings.Trim(tenant, "-")
	if tenant == "" {
		return "tenant"
	}
	return tenant
}
