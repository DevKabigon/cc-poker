# Commit Message
`feat: add buy-in foundation, supabase auth exchange, and frontend routing stack`

# Summary
- 바이인 선행 입장 흐름과 룸/테이블 시딩 구조를 정리했다.
- 지갑 정책을 게스트(2,000) / 인증 유저(10,000)로 분리했다.
- Supabase access token -> 백엔드 쿠키 세션 교환 엔드포인트를 추가했다.
- 프론트에 `react-router-dom`, `zustand`, `swr`를 도입해 라우팅/상태/헬스체크 폴링 구조를 만들었다.
- 인증/지갑 정책 관련 테스트를 추가하고 회귀 테스트를 통과했다.

# Verification
- `go test ./...`
- `pnpm -C web build`

