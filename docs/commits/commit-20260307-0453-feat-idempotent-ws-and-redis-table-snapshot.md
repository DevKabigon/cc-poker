# Commit Note - 2026-03-07 04:53 JST

## Commit Message
`feat: add idempotent ws handling and redis table snapshot restore`

## Summary
- WebSocket 이벤트에 `request_id` 기반 멱등 처리 로직을 추가함.
- 동일 요청 재수신 시 이전 응답을 재전송하여 중복 상태 변경을 방지함.
- Redis 기반 테이블 스냅샷 저장/복구 기능을 추가함.
- 서버 시작 시 스냅샷 복구 로직과 join/leave/disconnect 시점 저장 로직을 반영함.
- 프론트에서 WS 이벤트 전송 시 `request_id`를 포함하도록 수정함.
- 관련 테스트(멱등성/복구) 및 문서를 업데이트함.
