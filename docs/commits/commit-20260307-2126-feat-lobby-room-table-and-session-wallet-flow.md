# Commit

- 시간(JST): 2026-03-07 21:26
- 커밋 메시지: `feat: add lobby-room-table flow and session wallet sync`

## 요약
- 프론트 라우팅을 `로비(/lobby) -> 룸(/rooms/:roomId) -> 테이블(/tables/:tableId)` 구조로 분리했다.
- 룸/테이블 카탈로그 모델을 추가해 블라인드 룸과 테이블 선택 흐름을 정리했다.
- 새로고침 시 쿠키 기반 세션 자동 복구를 위해 `GET /v1/session/current` API와 프론트 부트스트랩을 추가했다.
- 테이블 `leave/disconnect/switch` 경로에서 남은 스택이 월렛으로 환급되도록 백엔드 로직을 보강했다.
- 지갑 UI 갱신 타이밍(leave 직후, wallet fetch 오류 표시)을 개선하고 관련 테스트/로그를 추가했다.
- `air` 설정을 최신 방식(`build.entrypoint`)으로 정리해 deprecation 경고를 제거했다.
