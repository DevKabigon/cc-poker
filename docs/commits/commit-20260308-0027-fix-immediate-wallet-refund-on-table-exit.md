# Commit

- 시간(JST): 2026-03-08 00:27
- 커밋 메시지: `fix: immediate wallet refund on table exit`

## 요약
- 테이블 페이지에서 룸 목록으로 이동할 때 환급이 지연되던 문제를 수정했다.
- `usePlayConsole` 훅의 unmount 시점에 WebSocket을 즉시 종료하도록 cleanup을 추가했다.
- 이탈 시 `disconnect_leave` 경로가 즉시 실행되어 월렛 환급이 바로 반영되도록 정리했다.
- 관련 작업 로그(`work-20260308-0021-29.md`)를 함께 반영했다.
