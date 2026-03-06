# Commit Note - 2026-03-07 04:36 JST

## Commit Message
`feat: add phase1 infra, ws broadcast, and react web console`

## Summary
- Docker Compose 기반 개발 인프라(Postgres/Redis) 구성을 추가함.
- WebSocket 허브를 도입해 테이블 스냅샷 브로드캐스트를 다중 클라이언트에 동기화하도록 개선함.
- 클라이언트 연결 종료 시 자동 퇴장 처리 및 브로드캐스트 반영 로직을 추가함.
- React + Vite 최소 클라이언트를 추가해 게스트 세션 발급/WS 연결/join/leave/스냅샷 확인이 가능하도록 구성함.
- 백엔드 테스트(`go test ./...`)와 프론트 빌드(`pnpm build`) 검증을 완료함.
