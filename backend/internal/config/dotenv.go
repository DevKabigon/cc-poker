package config

import (
	"os"
	"strings"
	"sync"
)

var dotenvOnce sync.Once

// loadDotEnvIfPresent는 .env 파일이 있을 때만 환경변수로 주입한다.
// 이미 셸에 설정된 환경변수는 덮어쓰지 않는다.
func loadDotEnvIfPresent() {
	dotenvOnce.Do(func() {
		for _, candidate := range []string{".env", "backend/.env"} {
			content, err := os.ReadFile(candidate)
			if err != nil {
				continue
			}
			parseAndApplyDotEnv(string(content))
			return
		}
	})
}

func parseAndApplyDotEnv(content string) {
	lines := strings.Split(content, "\n")
	for _, raw := range lines {
		line := strings.TrimSpace(raw)
		if line == "" || strings.HasPrefix(line, "#") {
			continue
		}

		if strings.HasPrefix(line, "export ") {
			line = strings.TrimSpace(strings.TrimPrefix(line, "export "))
		}

		eq := strings.Index(line, "=")
		if eq <= 0 {
			continue
		}

		key := strings.TrimSpace(line[:eq])
		if key == "" {
			continue
		}

		// 이미 셸에 설정된 값이 있으면 우선한다.
		if strings.TrimSpace(os.Getenv(key)) != "" {
			continue
		}

		value := strings.TrimSpace(line[eq+1:])
		value = strings.Trim(value, "\"")
		value = strings.Trim(value, "'")
		_ = os.Setenv(key, value)
	}
}
