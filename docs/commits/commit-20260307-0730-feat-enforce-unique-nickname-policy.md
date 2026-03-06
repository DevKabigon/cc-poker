# Commit Message
`feat: enforce unique nickname policy across session and persistence`

# Summary
- 닉네임을 전역 유니크로 강제했다.
  - 세션 생성 시 중복 닉네임 차단
  - DB 저장 시 닉네임 unique violation을 `ErrNicknameTaken`으로 처리
  - API 응답을 `409 Conflict`로 정규화
- `users(lower(nickname))` 유니크 인덱스 마이그레이션을 추가했다.
- 인증 닉네임 fallback을 사용자 ID 기반으로 보강했다.
- 관련 테스트를 업데이트하고 전체 테스트/빌드를 통과했다.

# Verification
- `go test ./...`
- `pnpm -C web build`

