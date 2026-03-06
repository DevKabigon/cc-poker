# Commit Note - 2026-03-07 05:38 JST

## Commit Message
`feat: add postgres persistence and backend dev watcher improvements`

## Summary
- Postgres 기반 영속화 계층(EventStore)과 초기 마이그레이션을 추가함.
- 게스트 세션 생성 시 `users`, `sessions`를 저장하고 테이블 join/leave/disconnect 이벤트를 `table_events`에 기록하도록 연결함.
- 백엔드 개발 스크립트를 개선해 코드 변경 시 자동 재시작과 기존 8080 프로세스 채택(adopt)을 지원하도록 수정함.
- Air 설정 및 로컬 개발 관련 설정(`.gitignore`, watcher 설정)을 보완함.
- 관련 work-log 문서를 추가함.
